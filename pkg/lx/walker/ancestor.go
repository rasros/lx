package walker

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadAncestorRules sets AncestorRules from the ignore files in the
// directories above dir, the absolute path the walk's filesystem is rooted
// at. It reads up to the enclosing git repository's root, or outside a
// repository up to the working directory when dir is below it. dir's own
// ignore files are left to the walk.
//
// It also sets RootIgnored when dir or a directory above it is ignored,
// checking each directory from the top down with the rules that apply at its
// level, BaseRules included, as git does.
func (w *Walker) LoadAncestorRules(dir string) {
	w.AncestorRules = nil
	w.RootIgnored = false
	dir = filepath.Clean(dir)
	root, ok := ancestorRoot(dir)
	if !ok || root == dir {
		return
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	levels := make([]int, len(w.BaseRules))
	cur := root
	for i, part := range parts {
		offset := strings.Join(parts[i:], "/") + "/"
		for _, r := range w.loadIgnoreFiles(os.DirFS(cur), ".") {
			r.PathOffset = offset
			r.Source = filepath.Join(cur, r.Source)
			w.AncestorRules = append(w.AncestorRules, r)
			levels = append(levels, i)
		}
		if !w.RootIgnored {
			w.RootIgnored = w.dirIgnored(parts, i, levels)
		}
		cur = filepath.Join(cur, part)
	}
}

// dirIgnored reports whether the directory parts[:i+1] is ignored. levels
// gives, for BaseRules followed by AncestorRules, the depth of the directory
// each rule was loaded from, so it can be matched relative to that directory.
func (w *Walker) dirIgnored(parts []string, i int, levels []int) bool {
	ignored := false
	for k, level := range levels {
		var r Rule
		if k < len(w.BaseRules) {
			r = w.BaseRules[k]
		} else {
			r = w.AncestorRules[k-len(w.BaseRules)]
			r.PathOffset = ""
		}
		if match(r, strings.Join(parts[level:i+1], "/"), true) {
			ignored = !r.Negate
		}
	}
	return ignored
}

// ancestorRoot picks the highest directory whose ignore files apply to dir.
func ancestorRoot(dir string) (string, bool) {
	if root, ok := findRepoRoot(dir); ok {
		return root, true
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return cwd, true
}

// findRepoRoot returns the nearest directory at or above dir that holds a
// .git entry. In a linked worktree .git is a file, which still marks the root.
func findRepoRoot(dir string) (string, bool) {
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
