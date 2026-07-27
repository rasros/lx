package walker

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path"
	"strings"
)

func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}

	if !info.IsDir() {
		if w.SkipHidden && isHiddenName(path.Base(root)) {
			if w.OnIgnore != nil {
				w.OnIgnore(root, "hidden path")
			}
			return nil
		}

		var localRules []Rule
		if w.IgnoreEnabled {
			dir := path.Dir(root)
			// Always load root rules.
			localRules = append(localRules, w.loadIgnoreFiles(fsys, ".")...)

			if dir != "." {
				parts := strings.Split(dir, "/")
				curr := ""
				for _, p := range parts {
					if curr == "" {
						curr = p
					} else {
						curr = curr + "/" + p
					}
					localRules = append(localRules, w.loadIgnoreFiles(fsys, curr)...)
				}
			}
		}

		effectiveRules := make([]Rule, 0, len(w.BaseRules)+len(localRules)+len(w.OverrideRules))
		effectiveRules = append(effectiveRules, w.BaseRules...)
		effectiveRules = append(effectiveRules, localRules...)
		effectiveRules = append(effectiveRules, w.OverrideRules...)

		isIgnored, reason := checkIgnore(root, info.IsDir(), effectiveRules, false, w.OnIgnore != nil)
		if isIgnored {
			if w.OnIgnore != nil {
				w.OnIgnore(root, reason)
			}
			return nil
		}
		return walkFn(root, dirEntryAdapter{info}, nil)
	}

	initialRules := make([]Rule, 0, len(w.BaseRules))
	initialRules = append(initialRules, w.BaseRules...)
	return w.recursiveWalk(fsys, root, initialRules, walkFn, false, []fs.FileInfo{info})
}

// resolveEntry reports how an entry should be treated, applying symlink policy.
// A symlink whose target cannot be stat'd is left as a file so the callback can
// surface the error.
func (w *Walker) resolveEntry(fsys fs.FS, path string, d fs.DirEntry, ancestors []fs.FileInfo) (isDir bool, target fs.FileInfo, skip bool, reason string) {
	if d.Type()&fs.ModeSymlink == 0 {
		return d.IsDir(), nil, false, ""
	}

	info, err := fs.Stat(fsys, path)
	if err != nil {
		// Broken link. It resolves to nothing, so file-symlink policy governs
		// it; otherwise leave it for the callback to report.
		if w.SkipFileSymlinks {
			return false, nil, true, "file symlink"
		}
		return false, nil, false, ""
	}

	if !info.IsDir() {
		if w.SkipFileSymlinks {
			return false, nil, true, "file symlink"
		}
		return false, nil, false, ""
	}

	if !w.FollowDirSymlinks {
		return false, nil, true, "directory symlink"
	}
	for _, a := range ancestors {
		if os.SameFile(a, info) {
			return true, info, true, "symlink cycle"
		}
	}
	return true, info, false, ""
}

func (w *Walker) recursiveWalk(fsys fs.FS, dir string, parentRules []Rule, walkFn fs.WalkDirFunc, parentIgnored bool, ancestors []fs.FileInfo) error {
	var localRules []Rule
	if w.IgnoreEnabled {
		localRules = w.loadIgnoreFiles(fsys, dir)
	}

	var mergedRules []Rule
	if len(localRules) == 0 {
		mergedRules = parentRules
	} else {
		mergedRules = make([]Rule, 0, len(parentRules)+len(localRules))
		mergedRules = append(mergedRules, parentRules...)
		mergedRules = append(mergedRules, localRules...)
	}

	var effectiveRules []Rule
	if len(w.OverrideRules) == 0 {
		effectiveRules = mergedRules
	} else {
		effectiveRules = make([]Rule, 0, len(mergedRules)+len(w.OverrideRules))
		effectiveRules = append(effectiveRules, mergedRules...)
		effectiveRules = append(effectiveRules, w.OverrideRules...)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}

	for _, d := range entries {
		childPath := d.Name()
		if dir != "." {
			childPath = path.Join(dir, d.Name())
		}

		if w.SkipHidden && isHiddenName(d.Name()) {
			if w.OnIgnore != nil {
				w.OnIgnore(childPath, "hidden path")
			}
			continue
		}

		isDir, target, skip, skipReason := w.resolveEntry(fsys, childPath, d, ancestors)
		if skip {
			if w.OnIgnore != nil {
				w.OnIgnore(childPath, skipReason)
			}
			continue
		}

		// Cycle detection needs every directory on the current path, not just
		// the followed links. Skipped entirely when not following, so ordinary
		// walks pay no extra stat.
		childAncestors := ancestors
		if isDir && w.FollowDirSymlinks {
			info := target
			if info == nil {
				info, _ = d.Info()
			}
			if info != nil {
				childAncestors = append(ancestors, info)
			}
		}

		isIgnored, reason := checkIgnore(childPath, isDir, effectiveRules, parentIgnored, w.OnIgnore != nil)

		if isDir {
			if isIgnored {
				if hasNestedException(childPath, effectiveRules) {
					if err := w.recursiveWalk(fsys, childPath, mergedRules, walkFn, true, childAncestors); err != nil {
						return err
					}
					continue
				}
				if w.OnIgnore != nil {
					w.OnIgnore(childPath, reason)
				}
				continue
			}

			entry := d
			if target != nil {
				entry = followedDirEntry{DirEntry: d, info: target}
			}
			if err := walkFn(childPath, entry, nil); err != nil {
				if err == fs.SkipDir {
					continue
				}
				return err
			}

			if err := w.recursiveWalk(fsys, childPath, mergedRules, walkFn, false, childAncestors); err != nil {
				return err
			}
			continue
		}

		if isIgnored {
			if w.OnIgnore != nil {
				w.OnIgnore(childPath, reason)
			}
			continue
		}
		if err := walkFn(childPath, d, nil); err != nil {
			return err
		}
	}
	return nil
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

func (w *Walker) loadIgnoreFiles(fsys fs.FS, dir string) []Rule {
	var rules []Rule
	files := []string{".gitignore", ".ignore", ".lxignore"}

	for _, name := range files {
		fp := name
		if dir != "." {
			fp = path.Join(dir, name)
		}

		data, err := fs.ReadFile(fsys, fp)
		if err == nil {
			sc := bufio.NewScanner(bytes.NewReader(data))
			var lines []string
			for sc.Scan() {
				lines = append(lines, sc.Text())
			}
			rules = append(rules, parseRules(lines, dir, name)...)
		}
	}
	return rules
}

// followedDirEntry presents a followed directory symlink as a directory, so
// callbacks branch on it the same way they branch on a real one. The embedded
// entry keeps the link's own name.
type followedDirEntry struct {
	fs.DirEntry
	info fs.FileInfo
}

func (f followedDirEntry) IsDir() bool                { return true }
func (f followedDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (f followedDirEntry) Info() (fs.FileInfo, error) { return f.info, nil }

type dirEntryAdapter struct {
	info fs.FileInfo
}

func (d dirEntryAdapter) Name() string               { return d.info.Name() }
func (d dirEntryAdapter) IsDir() bool                { return d.info.IsDir() }
func (d dirEntryAdapter) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d dirEntryAdapter) Info() (fs.FileInfo, error) { return d.info, nil }
