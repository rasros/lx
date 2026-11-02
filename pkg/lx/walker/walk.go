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
func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	rt := newRuleTree(fsTree{fsys}, w.opts.IgnoreEnabled)
	if info.IsDir() {
		_, err = w.walkRoot(rt, fsys, root, root, info, walkFn)
		return err
	}
	_, err = w.walkFile(rt, ".", root, root, info, walkFn)
	return err
}

// WalkPath walks absRoot, an absolute path on the cache's host, with the
// ignore rules of every directory above it, as ripgrep does: .ignore and
// .lxignore everywhere, and git's rules inside a repository, which start over
// at its root. Paths passed to fn are relative to absRoot when it is a
// directory; a file root is passed as its base name.
//
// Override rules match relative to absRoot, or for a file root relative to
// the working directory when the file is below it, and to the file's
// directory otherwise.
//
// skipped is the reason absRoot itself was skipped, empty when it was not.
func (w *Walker) WalkPath(absRoot string, fn fs.WalkDirFunc) (skipped string, err error) {
	h := w.cache.host
	absRoot = filepath.Clean(absRoot)
	info, err := h.Stat(absRoot)
	if err != nil {
		return "", fn(filepath.Base(absRoot), nil, err)
	}
	rt := w.cache.hostTree()
	if info.IsDir() {
		fsys := h.DirFS(absRoot)
		// The walk needs to search the directory, which stat'ing it alone
		// does not check.
		if info, err = fs.Stat(fsys, "."); err != nil {
			return "", fn(".", nil, err)
		}
		dir := absRoot
		if !w.opts.IgnoreEnabled {
			rt, dir = newRuleTree(fsTree{fsys}, false), "."
		}
		return w.walkRoot(rt, fsys, dir, ".", info, fn)
	}

	base, rel := filepath.Dir(absRoot), filepath.Base(absRoot)
	if r, ok := w.cache.ht.below(absRoot); ok {
		base, rel = w.cache.ht.cwd, filepath.ToSlash(r)
	}
	if !w.opts.IgnoreEnabled {
		rt, base = newRuleTree(fsTree{h.DirFS(base)}, false), "."
	}
	return w.walkFile(rt, base, rel, filepath.Base(absRoot), info, fn)
}

func (w *Walker) ignore(p, reason string) {
	if w.opts.OnIgnore != nil {
		w.opts.OnIgnore(p, reason)
	}
}

// walkRoot walks the directory dir of rt, which is root within fsys.
func (w *Walker) walkRoot(rt *ruleTree, fsys fs.FS, dir, root string, info fs.FileInfo, fn fs.WalkDirFunc) (string, error) {
	f := rt.frame(w, dir)
	if f.excluded {
		w.ignore(root, f.reason)
		return f.reason, nil
	}
	return "", w.walkDir(rt, fsys, f, root, f.ignored, f.reason, fn, []fs.FileInfo{info})
}

// walkFile checks the single file at rel below the directory base of rt: the
// frame of its directory, plus a check on the file. Override rules also apply
// to the directories between base and the file, as a walk from base would
// apply them. The file is passed to fn as name.
func (w *Walker) walkFile(rt *ruleTree, base, rel, name string, info fs.FileInfo, fn fs.WalkDirFunc) (string, error) {
	skip := func(reason string) (string, error) {
		w.ignore(name, reason)
		return reason, nil
	}
	f := rt.frame(w, base)
	if f.excluded {
		return skip(f.reason)
	}
	if w.opts.SkipHidden && isHiddenName(path.Base(rel)) {
		return skip("hidden path")
	}

	ignored, reason := f.ignored, f.reason
	if parent := path.Dir(rel); parent != "." {
		dir, walkRel := base, "."
		for _, part := range strings.Split(parent, "/") {
			child := rt.t.child(dir, part)
			walkRel = joinRel(walkRel, part)
			dir = child
			if len(w.override) == 0 {
				continue
			}
			ign, descend, r, own := judgeDir(f.rules, joinRel(f.node.rel, part), w.override, walkRel, ignored, reason)
			if ign {
				r = dirReason(rt.t.display(child), r, own)
				if !descend {
					return skip(r)
				}
			}
			ignored, reason = ign, r
			f = rt.frame(w, child)
		}
		if len(w.override) == 0 {
			// Without override rules the directory's frame holds the answer.
			if f = rt.frame(w, dir); f.excluded {
				return skip(f.reason)
			}
			ignored, reason = f.ignored, f.reason
		}
	}

	ignored, reason = checkIgnore(joinRel(f.node.rel, path.Base(rel)), false, f.rules, ignored, reason, true)
	ignored, reason = checkIgnore(rel, false, w.override, ignored, reason, true)
	if ignored {
		return skip(reason)
	}
	return "", fn(name, dirEntryAdapter{info}, nil)
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

// walkDir walks the directory at dir within fsys, whose frame is f. ignored
// and reason are the state carried from above, set when the directory is
// ignored but entered because a negation could re-include something below.
func (w *Walker) walkDir(rt *ruleTree, fsys fs.FS, f *frame, dir string, ignored bool, reason string, walkFn fs.WalkDirFunc, ancestors []fs.FileInfo) error {
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

		if w.opts.SkipHidden && isHiddenName(d.Name()) {
			w.ignore(childPath, "hidden path")
			continue
		}

		isDir, target, skip, skipReason := w.resolveEntry(fsys, childPath, d, ancestors)
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
			ign, descend, r, own := judgeDir(f.rules, rel, w.override, childPath, ignored, reason)
			if ign && !descend {
				w.ignore(childPath, r)
				continue
			}
			if ign {
				r = dirReason(rt.t.display(childDir), r, own)
			} else {
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
			}
			if err := w.walkDir(rt, fsys, rt.frame(w, childDir), childPath, ign, r, walkFn, childAncestors); err != nil {
				return err
			}
			continue
		}

		ign, r := checkIgnore(rel, false, f.rules, ignored, reason, wantReason)
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
