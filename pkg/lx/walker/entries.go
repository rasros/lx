package walker

import (
	"path"
	"strings"
)

// Entries judges the paths of a tree read as a sequence of entries, such as
// an archive, as Walk would judge them, without ignore files. A path below a
// skipped directory is skipped too. It is not safe for concurrent use.
type Entries struct {
	w  *Walker
	rt *ruleTree
	a  Anchor
	// dirs holds whether each directory judged so far is kept.
	dirs map[string]bool
	// shown judges as Entries does without hidden-file skipping.
	shown *Entries
}

// Entries returns a judge of the entries of one tree.
func (w *Walker) Entries() *Entries {
	return &Entries{
		w:    w,
		rt:   newRuleTree(fsTree{}, false),
		a:    Anchor{prefix: w.opts.OverridePrefix},
		dirs: map[string]bool{},
	}
}

// Keep reports whether a walk of the tree keeps the entry at p, a clean
// slash-separated path from the tree's root.
func (e *Entries) Keep(p string, isDir bool) bool {
	if isDir {
		return e.keepDir(p)
	}
	dir, name := splitEntry(p)
	if dir != "." && !e.keepDir(dir) {
		return false
	}
	asDir := e.w.opts.MatchAsDir != nil && e.w.opts.MatchAsDir(name)
	if skip, reason := e.w.judge(e.rt.frame(e.w, dir), name, p, e.a, asDir); skip {
		e.w.ignore(p, reason)
		return false
	}
	return true
}

// KeepShown is Keep without hidden-file skipping, for a named file.
func (e *Entries) KeepShown(p string, isDir bool) bool {
	if !e.w.opts.SkipHidden {
		return e.Keep(p, isDir)
	}
	if e.shown == nil {
		w := *e.w
		w.opts.SkipHidden = false
		e.shown = &Entries{w: &w, rt: e.rt, a: e.a, dirs: map[string]bool{}}
	}
	return e.shown.Keep(p, isDir)
}

func (e *Entries) keepDir(d string) bool {
	if kept, ok := e.dirs[d]; ok {
		return kept
	}
	parent, name := splitEntry(d)
	kept := parent == "." || e.keepDir(parent)
	if kept {
		if skip, reason := e.w.judge(e.rt.frame(e.w, parent), name, d, e.a, true); skip {
			e.w.ignore(d, reason)
			kept = false
		}
	}
	e.dirs[d] = kept
	return kept
}

// splitEntry splits p into its directory, "." at the root, and its name.
func splitEntry(p string) (dir, name string) {
	dir, name = path.Split(p)
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		dir = "."
	}
	return dir, name
}
