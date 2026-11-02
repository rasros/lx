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
func (w *Walker) LoadAncestorRules(dir string) {
	w.AncestorRules = nil
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
	cur := root
	for i, part := range parts {
		offset := strings.Join(parts[i:], "/") + "/"
		for _, r := range w.loadIgnoreFiles(os.DirFS(cur), ".") {
			r.PathOffset = offset
			r.Source = filepath.Join(cur, r.Source)
			w.AncestorRules = append(w.AncestorRules, r)
		}
		cur = filepath.Join(cur, part)
	}
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
