package walker

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path"
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
// When dir is itself a repository root there is nothing above it to load:
// the walk reads that repository's info/exclude at its boundary.
//
// It also sets RootIgnored when dir or a directory above it is ignored,
// checking each directory from the top down with the rules that apply at its
// level, BaseRules included, as git does.
func (w *Walker) LoadAncestorRules(dir string) {
	w.AncestorRules = nil
	w.RootIgnored = false
	dir = filepath.Clean(dir)

	root, isRepo, ok := ancestorRoot(dir)
	if !ok || root == dir {
		return
	}
	parts, ok := relParts(root, dir)
	if !ok {
		return
	}

	levels := make([]int, len(w.BaseRules))
	if isRepo {
		offset := strings.Join(parts, "/") + "/"
		for _, r := range infoExcludeRules(root, ".") {
			r.PathOffset = offset
			w.AncestorRules = append(w.AncestorRules, r)
			levels = append(levels, 0)
		}
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

// repoRules reports whether the walk-relative directory dir is the root of a
// git repository, and if so returns the rules that replace everything
// inherited from outside it: BaseRules, which apply to every repository, and
// the repository's own info/exclude. Git never applies one repository's
// ignore rules inside another, including submodules.
func (w *Walker) repoRules(fsys fs.FS, dir string) ([]Rule, bool) {
	if _, err := fs.Stat(fsys, path.Join(dir, ".git")); err != nil {
		return nil, false
	}
	rules := make([]Rule, 0, len(w.BaseRules))
	rules = append(rules, w.BaseRules...)
	if w.FSRoot != "" {
		rules = append(rules, infoExcludeRules(filepath.Join(w.FSRoot, filepath.FromSlash(dir)), dir)...)
	} else {
		p := path.Join(dir, ".git", "info", "exclude")
		data, err := fs.ReadFile(fsys, p)
		logReadError(p, err)
		if err == nil {
			rules = append(rules, parseRuleFile(data, dir, p)...)
		}
	}
	return rules, true
}

// infoExcludeRules reads the info/exclude rules of the repository rooted at
// the absolute path repo, scoped to basePath within the walk.
func infoExcludeRules(repo, basePath string) []Rule {
	gitDir, ok := resolveGitDir(repo)
	if !ok {
		return nil
	}
	p := filepath.Join(commonGitDir(gitDir), "info", "exclude")
	data, err := os.ReadFile(p)
	logReadError(p, err)
	if err != nil {
		return nil
	}
	return parseRuleFile(data, basePath, p)
}

// parseRuleFile parses an ignore file. It splits on newlines rather than
// scanning, so no line length limit can drop the rules after a long line.
func parseRuleFile(data []byte, basePath, source string) []Rule {
	return parseRules(strings.Split(string(data), "\n"), basePath, source)
}

// logReadError records why a rule file that exists could not be read, so a
// silently missing set of rules shows up in --debug output.
func logReadError(path string, err error) {
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Debug("Failed to read ignore rules", "path", path, "error", err)
	}
}

// resolveGitDir returns the git directory for the repository rooted at repo.
// In linked worktrees and submodules .git is a file holding a "gitdir:" line.
func resolveGitDir(repo string) (string, bool) {
	dotGit := filepath.Join(repo, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		logReadError(dotGit, err)
		return "", false
	}
	if info.IsDir() {
		return dotGit, true
	}

	data, err := os.ReadFile(dotGit)
	if err != nil {
		logReadError(dotGit, err)
		return "", false
	}
	line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	target, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		slog.Debug("Ignoring .git file without a gitdir line", "path", dotGit)
		return "", false
	}
	return resolveRelative(repo, strings.TrimSpace(target)), true
}

// commonGitDir returns the directory shared by all worktrees of gitDir,
// which holds info/exclude. Without a commondir file it is gitDir itself.
func commonGitDir(gitDir string) string {
	p := filepath.Join(gitDir, "commondir")
	data, err := os.ReadFile(p)
	if err != nil {
		logReadError(p, err)
		return gitDir
	}
	common := strings.TrimSpace(string(data))
	if common == "" {
		return gitDir
	}
	return resolveRelative(gitDir, common)
}

// resolveRelative resolves target against base the way the filesystem does.
// filepath.Join would collapse ".." textually, which goes wrong when base is
// reached through a symlink, so base is made real first.
func resolveRelative(base, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	return filepath.Join(base, target)
}

// relParts splits dir's path relative to root into slash-free components.
func relParts(root, dir string) ([]string, bool) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return nil, false
	}
	return strings.Split(filepath.ToSlash(rel), "/"), true
}

// ancestorRoot picks the highest directory whose ignore files apply to dir:
// the enclosing repository's root, or outside a repository the working
// directory when dir is below it. isRepo reports which.
func ancestorRoot(dir string) (root string, isRepo, ok bool) {
	if root, ok := findRepoRoot(dir); ok {
		return root, true, true
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, false
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, false
	}
	return cwd, false, true
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
