package walker

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// LoadAncestorRules sets AncestorRules from the ignore rules that live
// outside dir, the absolute path the walk's filesystem is rooted at:
//
//   - the enclosing git repository's info/exclude file, which git ranks below
//     every .gitignore, so it comes first;
//   - the ignore files in the directories above dir, up to the repository
//     root, or outside a repository up to the working directory when dir is
//     below it.
//
// dir's own ignore files are left to the walk.
//
// It also sets RootIgnored when dir or a directory above it is ignored,
// checking each directory from the top down with the rules that apply at its
// level, BaseRules included, as git does.
func (w *Walker) LoadAncestorRules(dir string) {
	w.AncestorRules = nil
	w.RootIgnored = false
	dir = filepath.Clean(dir)

	levels := make([]int, len(w.BaseRules))
	if repo, ok := findRepoRoot(dir); ok {
		for _, r := range loadInfoExclude(repo, dir) {
			w.AncestorRules = append(w.AncestorRules, r)
			levels = append(levels, 0)
		}
	}

	root, ok := ancestorRoot(dir)
	if !ok || root == dir {
		return
	}
	parts, ok := relParts(root, dir)
	if !ok {
		return
	}

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

// loadInfoExclude reads the info/exclude rules of the repository rooted at
// repo, offset so they match paths relative to dir.
func loadInfoExclude(repo, dir string) []Rule {
	gitDir, ok := resolveGitDir(repo)
	if !ok {
		return nil
	}
	path := filepath.Join(commonGitDir(gitDir), "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	rules := parseRules(lines, ".", path)

	if repo != dir {
		parts, ok := relParts(repo, dir)
		if !ok {
			return nil
		}
		offset := strings.Join(parts, "/") + "/"
		for i := range rules {
			rules[i].PathOffset = offset
		}
	}
	return rules
}

// resolveGitDir returns the git directory for the repository rooted at repo.
// In linked worktrees and submodules .git is a file holding a "gitdir:" line.
func resolveGitDir(repo string) (string, bool) {
	dotGit := filepath.Join(repo, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return dotGit, true
	}

	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	target, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return "", false
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(repo, target)
	}
	return filepath.Clean(target), true
}

// commonGitDir returns the directory shared by all worktrees of gitDir,
// which holds info/exclude. Without a commondir file it is gitDir itself.
func commonGitDir(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return gitDir
	}
	common := strings.TrimSpace(string(data))
	if common == "" {
		return gitDir
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	return filepath.Clean(common)
}

// relParts splits dir's path relative to root into slash-free components.
func relParts(root, dir string) ([]string, bool) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return nil, false
	}
	return strings.Split(filepath.ToSlash(rel), "/"), true
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
