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
// Paths passed to walkFn are relative to fsys, and so are overrides, after
// Options.OverridePrefix. Use WalkPath for the host filesystem.
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
	rt := newRuleTree(fsTree{fsys}, w.opts.IgnoreEnabled)
	return w.walkDir(rt, fsys, rt.frame(w, root), root, Anchor{prefix: w.opts.OverridePrefix}, walkFn, []fs.FileInfo{info})
}

// WalkPath walks root, a path on the cache's host named from its working
// directory or absolutely, with the ignore rules of every directory above
// it, as ripgrep does: .ignore and .lxignore everywhere, and git's rules
// inside a repository, which start over at its root. Paths passed to fn are
// relative to root when it is a directory; a file root is passed as its base
// name. As ripgrep's -g globs, overrides match the path as root was named,
// with the working directory stripped from the front of an absolute one.
//
// As in ripgrep, root itself is never filtered: a file is passed to fn and a
// directory walked, whatever the rules say about it or the directories above
// it. The entries below a directory are filtered as usual.
func (w *Walker) WalkPath(root string, fn fs.WalkDirFunc) error {
	h := w.cache.host
	absRoot := filepath.Clean(root)
	if !filepath.IsAbs(absRoot) {
		w.cache.hostTree()
		absRoot = filepath.Join(w.cache.wd, absRoot)
	}
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
	a := w.cache.Anchor(root)
	if !w.opts.IgnoreEnabled {
		rt := newRuleTree(fsTree{fsys}, false)
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

// judge decides whether the walk skips the entry name, at childPath within
// the walked fs.FS, in the directory of frame f, and why. As in ripgrep, the
// overrides, matched at the path a gives it, come first; then the ignore
// rules; then hidden-file skipping, which a rule keeping the entry lifts.
// In whitelist mode a directory no include can match below is skipped too.
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
	if isDir && w.overrides != nil && !w.overrides.reachBelow(ovPath) && !a.leadsToWD(ovPath) {
		return true, "no -i glob can match below it"
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

		// Cycle detection needs every directory on the current path, not just
		// the followed links. Skipped entirely when not following, so ordinary
		// walks pay no extra stat.
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
