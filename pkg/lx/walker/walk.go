package walker

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Walk walks root within fsys, which stands alone: ignore files apply from
// its root down. Paths passed to walkFn, and matched by overrides after
// Options.OverridePrefix, are relative to fsys. As in ripgrep, root itself
// is never filtered. Use WalkPath for the host filesystem.
func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	if !info.IsDir() {
		return walkFn(root, dirEntryAdapter{info}, nil)
	}
	t := newFSTree(fsys, w.log)
	t.repoRoot = w.opts.RepoRoot
	rt := newRuleTree(t, w.opts.IgnoreEnabled)
	return w.walkDir(rt, fsys, rt.frame(w, root), root, Anchor{prefix: w.opts.OverridePrefix}, walkFn, []fs.FileInfo{info})
}

// WalkPath walks root on the host with the ignore rules of every
// directory above it, as ripgrep does. Paths passed to fn are relative to a
// directory root; a file root is passed as its base name. Overrides match
// the path as root was named. As in ripgrep, root itself is never filtered.
func (w *Walker) WalkPath(root string, fn fs.WalkDirFunc) error {
	absRoot := filepath.Clean(root)
	if !filepath.IsAbs(absRoot) {
		w.cache.hostTree()
		absRoot = filepath.Join(w.cache.wd, absRoot)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return fn(filepath.Base(absRoot), nil, err)
	}
	if !info.IsDir() {
		return fn(filepath.Base(absRoot), dirEntryAdapter{info}, nil)
	}
	fsys := os.DirFS(absRoot)
	// Stat "." to check the directory can be searched, not just stat'd.
	if info, err = fs.Stat(fsys, "."); err != nil {
		return fn(".", nil, err)
	}
	a := w.cache.Anchor(root)
	if !w.opts.IgnoreEnabled {
		rt := newRuleTree(newFSTree(fsys, w.log), false)
		return w.walkDir(rt, fsys, rt.frame(w, "."), ".", a, fn, []fs.FileInfo{info})
	}
	// Like ripgrep, the rules of the directories above a symlinked root are
	// those above its target.
	rt := w.cache.hostTree()
	return w.walkDir(rt, fsys, rt.frame(w, w.cache.ht.real(absRoot)), ".", a, fn, []fs.FileInfo{info})
}

func (w *Walker) ignore(p, reason string) {
	if w.opts.OnIgnore != nil {
		w.opts.OnIgnore(p, reason)
	}
}

// resolveEntry applies symlink policy to an entry. A broken symlink is left
// as a file so the callback can report it.
func (w *Walker) resolveEntry(fsys fs.FS, path string, d fs.DirEntry, ancestors []fs.FileInfo) (isDir bool, target fs.FileInfo, skip bool, reason string) {
	if d.Type()&fs.ModeSymlink == 0 {
		return d.IsDir(), nil, false, ""
	}

	info, err := fs.Stat(fsys, path)
	if err != nil {
		// A broken link falls under file-symlink policy.
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

// judge decides whether the walk skips an entry, and why. As in ripgrep,
// overrides come first, then ignore rules, then hidden-file skipping, which
// a rule keeping the entry lifts.
func (w *Walker) judge(f *frame, name, childPath string, a Anchor, isDir bool) (bool, string) {
	wantReason := w.opts.OnIgnore != nil
	m := matchNone
	ovPath := ""
	if w.overrides != nil {
		ovPath = a.Path(childPath)
		var reason string
		if m, reason = w.overrides.match(ovPath, isDir, wantReason); m == matchIgnore {
			return true, reason
		}
	}
	if m == matchNone {
		fm, reason := lastMatch(joinRel(f.node.rel, name), isDir, f.rules, wantReason)
		if fm == matchIgnore {
			return true, reason
		}
		if fm == matchNone && w.opts.SkipHidden && isHiddenName(name) {
			return true, "hidden path"
		}
	}
	if isDir && w.overrides != nil && !w.overrides.reachBelow(ovPath) {
		return true, "no include glob can match below it"
	}
	return false, ""
}

// walkDir walks the directory at dir within fsys, whose frame is f. a names
// the paths within fsys for the overrides.
func (w *Walker) walkDir(rt *ruleTree, fsys fs.FS, f *frame, dir string, a Anchor, walkFn fs.WalkDirFunc, ancestors []fs.FileInfo) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}

	for _, d := range entries {
		childPath := d.Name()
		if dir != "." {
			childPath = path.Join(dir, d.Name())
		}

		isDir, target, skip, skipReason := w.resolveEntry(fsys, childPath, d, ancestors)
		if skip {
			w.ignore(childPath, skipReason)
			continue
		}
		asDir := isDir || (w.opts.MatchAsDir != nil && w.opts.MatchAsDir(d.Name()))
		if skip, reason := w.judge(f, d.Name(), childPath, a, asDir); skip {
			w.ignore(childPath, reason)
			continue
		}

		if !isDir {
			if err := walkFn(childPath, d, nil); err != nil {
				return err
			}
			continue
		}

		// Cycle detection needs every directory on the current path; walks
		// that do not follow links skip the extra stat.
		childAncestors := ancestors
		if w.opts.FollowDirSymlinks {
			info := target
			if info == nil {
				info, _ = d.Info()
			}
			if info != nil {
				childAncestors = append(ancestors, info)
			}
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
		childDir := rt.t.child(f.node.dir, d.Name())
		if err := w.walkDir(rt, fsys, rt.frame(w, childDir), childPath, a, walkFn, childAncestors); err != nil {
			return err
		}
	}
	return nil
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// followedDirEntry presents a followed directory symlink as a directory,
// keeping the link's own name.
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
