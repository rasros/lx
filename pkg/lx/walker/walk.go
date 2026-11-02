package walker

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Walk walks root within fsys, which stands alone: ignore files apply from
// its root down, and a .git file pointing outside it cannot be followed.
// Paths passed to walkFn, and override rules, are relative to fsys. Use
// WalkPath for the host filesystem.
//
// As in ripgrep, root itself is never filtered: a file is passed to walkFn
// and a directory walked, whatever the rules say about it.
func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	if !info.IsDir() {
		return walkFn(root, dirEntryAdapter{info}, nil)
	}
	return w.walkRoot(newRuleTree(fsTree{fsys}, w.opts.IgnoreEnabled), fsys, root, root, info, walkFn)
}

// WalkPath walks absRoot, an absolute path on the cache's host, with the
// ignore rules of every directory above it, as ripgrep does: .ignore and
// .lxignore everywhere, and git's rules inside a repository, which start over
// at its root. Paths passed to fn are relative to absRoot when it is a
// directory; a file root is passed as its base name. Override rules match
// relative to absRoot.
//
// As in ripgrep, absRoot itself is never filtered: a file is passed to fn
// and a directory walked, whatever the rules say about it or the directories
// above it. The entries below a directory are filtered as usual.
func (w *Walker) WalkPath(absRoot string, fn fs.WalkDirFunc) error {
	h := w.cache.host
	absRoot = filepath.Clean(absRoot)
	info, err := h.Stat(absRoot)
	if err != nil {
		return fn(filepath.Base(absRoot), nil, err)
	}
	if !info.IsDir() {
		return fn(filepath.Base(absRoot), dirEntryAdapter{info}, nil)
	}
	fsys := h.DirFS(absRoot)
	// The walk needs to search the directory, which stat'ing it alone does
	// not check.
	if info, err = fs.Stat(fsys, "."); err != nil {
		return fn(".", nil, err)
	}
	if !w.opts.IgnoreEnabled {
		return w.walkRoot(newRuleTree(fsTree{fsys}, false), fsys, ".", ".", info, fn)
	}
	// Like ripgrep, the rules of the directories above a symlinked root are
	// those above its target.
	rt := w.cache.hostTree()
	return w.walkRoot(rt, fsys, w.cache.ht.real(absRoot), ".", info, fn)
}

func (w *Walker) ignore(p, reason string) {
	if w.opts.OnIgnore != nil {
		w.opts.OnIgnore(p, reason)
	}
}

// walkRoot walks the directory dir of rt, which is root within fsys. The
// directory itself is not judged, only the entries below it.
func (w *Walker) walkRoot(rt *ruleTree, fsys fs.FS, dir, root string, info fs.FileInfo, fn fs.WalkDirFunc) error {
	return w.walkDir(rt, fsys, rt.frame(w, dir), root, fn, []fs.FileInfo{info})
}

// whitelisted reports whether a negation is the rule that decides the entry
// at rel in the directory of frame f, at walkRel for the override rules. As
// in ripgrep, that includes a hidden entry.
func (w *Walker) whitelisted(f *frame, rel, walkRel string, isDir bool) bool {
	matched, negated := whitelisted(rel, isDir, f.rules)
	if m, n := whitelisted(walkRel, isDir, w.override); m {
		matched, negated = true, n
	}
	return matched && negated
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
		if w.opts.SkipFileSymlinks {
			return false, nil, true, "file symlink"
		}
		return false, nil, false, ""
	}

	if !info.IsDir() {
		if w.opts.SkipFileSymlinks {
			return false, nil, true, "file symlink"
		}
		return false, nil, false, ""
	}

	if !w.opts.FollowDirSymlinks {
		return false, nil, true, "directory symlink"
	}
	for _, a := range ancestors {
		if os.SameFile(a, info) {
			return true, info, true, "symlink cycle"
		}
	}
	return true, info, false, ""
}

// walkDir walks the directory at dir within fsys, whose frame is f.
func (w *Walker) walkDir(rt *ruleTree, fsys fs.FS, f *frame, dir string, walkFn fs.WalkDirFunc, ancestors []fs.FileInfo) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}
	wantReason := w.opts.OnIgnore != nil

	for _, d := range entries {
		childPath := d.Name()
		if dir != "." {
			childPath = path.Join(dir, d.Name())
		}

		hidden := w.opts.SkipHidden && isHiddenName(d.Name())
		if hidden && d.Type()&fs.ModeSymlink == 0 && !w.whitelisted(f, joinRel(f.node.rel, d.Name()), childPath, d.IsDir()) {
			w.ignore(childPath, "hidden path")
			continue
		}

		isDir, target, skip, skipReason := w.resolveEntry(fsys, childPath, d, ancestors)
		if hidden && !skip && d.Type()&fs.ModeSymlink != 0 && !w.whitelisted(f, joinRel(f.node.rel, d.Name()), childPath, isDir) {
			skip, skipReason = true, "hidden path"
		}
		if skip {
			w.ignore(childPath, skipReason)
			continue
		}

		// Cycle detection needs every directory on the current path, not just
		// the followed links. Skipped entirely when not following, so ordinary
		// walks pay no extra stat.
		childAncestors := ancestors
		if isDir && w.opts.FollowDirSymlinks {
			info := target
			if info == nil {
				info, _ = d.Info()
			}
			if info != nil {
				childAncestors = append(ancestors, info)
			}
		}

		rel := joinRel(f.node.rel, d.Name())
		if isDir {
			childDir := rt.t.child(f.node.dir, d.Name())
			if ign, r := judgeDir(f.rules, rel, w.override, childPath); ign {
				w.ignore(childPath, r)
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
			if err := w.walkDir(rt, fsys, rt.frame(w, childDir), childPath, walkFn, childAncestors); err != nil {
				return err
			}
			continue
		}

		ign, r := checkIgnore(rel, false, f.rules, false, "", wantReason)
		ign, r = checkIgnore(childPath, false, w.override, ign, r, wantReason)
		if ign {
			w.ignore(childPath, r)
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
