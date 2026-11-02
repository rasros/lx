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
	var g gitFS = osGitFS{}
	repo := filepath.Join(w.FSRoot, filepath.FromSlash(dir))
	if w.FSRoot == "" {
		g, repo = subGitFS{fsys}, dir
	}
	gitDir, ok := findGitDir(g, repo)
	if !ok {
		return nil, false
	}
	rules := make([]Rule, 0, len(w.BaseRules))
	rules = append(rules, w.BaseRules...)
	rules = append(rules, infoExclude(g, gitDir, dir)...)
	return rules, true
}

// infoExcludeRules reads the info/exclude rules of the repository rooted at
// the absolute path repo, scoped to basePath within the walk.
func infoExcludeRules(repo, basePath string) []Rule {
	gitDir, ok := resolveGitDir(repo)
	if !ok {
		return nil
	}
	return infoExclude(osGitFS{}, gitDir, basePath)
}

// infoExclude reads the info/exclude rules shared by the worktrees of gitDir,
// scoped to basePath within the walk.
// An empty gitDir, one the walk cannot reach, has none.
func infoExclude(g gitFS, gitDir, basePath string) []Rule {
	if gitDir == "" {
		return nil
	}
	common, ok := commonDir(g, gitDir)
	if !ok {
		return nil
	}
	p := g.Join(common, "info", "exclude")
	data, err := g.ReadFile(p)
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

// gitFS is the filesystem a git directory is looked up in: the host
// filesystem, or a walked fs.FS that has no host path.
type gitFS interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadLink(name string) (string, error)
	ReadFile(name string) ([]byte, error)
	Join(elem ...string) string
	// Resolve resolves a path read from a .git or commondir file against
	// base, reporting false when it cannot be reached.
	Resolve(base, target string) (string, bool)
}

// osGitFS looks git directories up on the host filesystem.
type osGitFS struct{}

func (osGitFS) Stat(name string) (fs.FileInfo, error)  { return os.Stat(name) }
func (osGitFS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }
func (osGitFS) ReadLink(name string) (string, error)   { return os.Readlink(name) }
func (osGitFS) ReadFile(name string) ([]byte, error)   { return os.ReadFile(name) }
func (osGitFS) Join(elem ...string) string             { return filepath.Join(elem...) }
func (osGitFS) Resolve(base, target string) (string, bool) {
	return resolveRelative(base, target), true
}

// subGitFS looks git directories up within a walked fs.FS. Paths that leave
// it, absolute or through "..", cannot be followed.
type subGitFS struct{ fsys fs.FS }

func (g subGitFS) Stat(name string) (fs.FileInfo, error)  { return fs.Stat(g.fsys, name) }
func (g subGitFS) Lstat(name string) (fs.FileInfo, error) { return fs.Lstat(g.fsys, name) }
func (g subGitFS) ReadLink(name string) (string, error)   { return fs.ReadLink(g.fsys, name) }
func (g subGitFS) ReadFile(name string) ([]byte, error)   { return fs.ReadFile(g.fsys, name) }
func (subGitFS) Join(elem ...string) string               { return path.Join(elem...) }
func (subGitFS) Resolve(base, target string) (string, bool) {
	target = filepath.ToSlash(target)
	if path.IsAbs(target) || filepath.IsAbs(target) {
		return "", false
	}
	p := path.Join(base, target)
	return p, fs.ValidPath(p)
}

// resolveGitDir returns the git directory for the repository rooted at the
// absolute path repo, reporting false when repo is not a repository root.
func resolveGitDir(repo string) (string, bool) {
	return findGitDir(osGitFS{}, repo)
}

// findGitDir returns the git directory of the repository rooted at repo. Like
// git, it accepts a .git directory only when it looks like a git directory,
// and a .git file only when its "gitdir:" line, as used by linked worktrees
// and submodules, points at one. Anything else named .git is not a repository.
//
// A gitdir the walked filesystem cannot reach, absolute or outside it, cannot
// be checked. The .git file still marks a repository boundary, as it did in
// the repository the walk came from, and the git directory is returned empty.
func findGitDir(g gitFS, repo string) (string, bool) {
	dotGit := g.Join(repo, ".git")
	info, err := g.Stat(dotGit)
	if err != nil {
		logReadError(dotGit, err)
		return "", false
	}
	gitDir := dotGit
	if !info.IsDir() {
		data, err := g.ReadFile(dotGit)
		if err != nil {
			logReadError(dotGit, err)
			return "", false
		}
		line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
		target, ok := strings.CutPrefix(line, "gitdir:")
		target = strings.TrimSpace(target)
		if !ok || target == "" {
			slog.Debug("Ignoring .git file without a gitdir line", "path", dotGit)
			return "", false
		}
		if gitDir, ok = g.Resolve(repo, target); !ok {
			slog.Debug("Keeping .git file whose gitdir cannot be followed as a repository root", "path", dotGit)
			return "", true
		}
	}
	if !isGitDir(g, gitDir) {
		slog.Debug("Ignoring .git that is not a valid git directory", "path", dotGit, "gitdir", gitDir)
		return "", false
	}
	return gitDir, true
}

// isGitDir reports whether dir looks like a git directory, following git's
// is_git_directory: a valid HEAD, and objects and refs directories in the
// directory shared by all worktrees. A shared directory the walked filesystem
// cannot reach is taken on trust.
func isGitDir(g gitFS, dir string) bool {
	if !validHead(g, g.Join(dir, "HEAD")) {
		return false
	}
	common, ok := commonDir(g, dir)
	if !ok {
		return true
	}
	for _, name := range []string{"objects", "refs"} {
		info, err := g.Stat(g.Join(common, name))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

// validHead reports whether the file at p is a HEAD git accepts, following
// git's validate_headref: a symlink into refs/, which need not exist on an
// unborn branch or after the refs are packed, or a file holding a symbolic
// ref into refs/ or starting with an object id.
func validHead(g gitFS, p string) bool {
	info, err := g.Lstat(p)
	if err != nil {
		return false
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := g.ReadLink(p)
		return err == nil && strings.HasPrefix(filepath.ToSlash(target), "refs/")
	}
	data, err := g.ReadFile(p)
	return err == nil && validHeadContent(data)
}

// validHeadContent reports whether data is a HEAD git accepts: a symbolic ref
// into refs/, or text starting with a full SHA-1 or SHA-256 object id.
func validHeadContent(data []byte) bool {
	s := string(data)
	if ref, ok := strings.CutPrefix(s, "ref:"); ok {
		return strings.HasPrefix(strings.TrimLeft(ref, " \t\r\n"), "refs/")
	}
	return hexPrefix(s, 64) || hexPrefix(s, 40)
}

// hexPrefix reports whether s starts with n hex digits.
func hexPrefix(s string, n int) bool {
	if len(s) < n {
		return false
	}
	for _, c := range s[:n] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// commonGitDir returns the directory shared by all worktrees of gitDir,
// which holds info/exclude. Without a commondir file it is gitDir itself.
func commonGitDir(gitDir string) string {
	common, _ := commonDir(osGitFS{}, gitDir)
	return common
}

// commonDir returns the directory shared by all worktrees of gitDir,
// reporting false when its commondir file points where g cannot reach.
func commonDir(g gitFS, gitDir string) (string, bool) {
	p := g.Join(gitDir, "commondir")
	data, err := g.ReadFile(p)
	if err != nil {
		logReadError(p, err)
		return gitDir, true
	}
	common := strings.TrimSpace(string(data))
	if common == "" {
		return gitDir, true
	}
	return g.Resolve(gitDir, common)
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

// findRepoRoot returns the nearest directory at or above dir that is a git
// repository root, see findGitDir. In a linked worktree .git is a file, which
// still marks the root.
func findRepoRoot(dir string) (string, bool) {
	for {
		if _, ok := resolveGitDir(dir); ok {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
