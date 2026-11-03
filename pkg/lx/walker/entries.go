package walker

import (
	"path"
	"strings"
)

// Entries judges the paths of a tree that is read as a sequence of entries
// instead of walked, such as an archive, as Walk judges them in an fs.FS
// holding the tree: the overrides match them after Options.OverridePrefix,
// and the base rules and hidden-file skipping apply, but there are no
// ignore files to read. A path below a directory the walk would skip is
// skipped with it, whether or not the directory has an entry of its own.
// It is not safe for concurrent use.
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
// slash-separated path from the tree's root that is a directory when isDir.
// As in a walk, Options.OnIgnore is told about a skipped path once, and not
// about the paths below a skipped directory.
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

// KeepShown reports whether a walk keeps the file at p as Keep does, but
// never skips it as hidden: it stands for a file that was named, such as
// the content of a named compressed file.
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
