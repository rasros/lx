package walker

import (
	"errors"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
)

// infoExclude reads the info/exclude rules shared by the worktrees of gitDir,
// scoped to basePath.
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
	logReadError(g.logger(), p, err)
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
func logReadError(log *slog.Logger, path string, err error) {
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Debug("Failed to read ignore rules", "path", path, "error", err)
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
	// logger is where lookups report what they skip.
	logger() *slog.Logger
}

// hostGitFS looks git directories up on the host filesystem.
type hostGitFS struct {
	Host
	log *slog.Logger
}

func (g hostGitFS) logger() *slog.Logger { return g.log }

func (hostGitFS) Join(elem ...string) string { return filepath.Join(elem...) }
func (hostGitFS) Resolve(base, target string) (string, bool) {
	return resolveRelative(base, target), true
}

// subGitFS looks git directories up within a walked fs.FS. Paths that leave
// it, absolute or through "..", cannot be followed.
type subGitFS struct {
	fsys fs.FS
	log  *slog.Logger
}

func (g subGitFS) logger() *slog.Logger { return g.log }

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
		logReadError(g.logger(), dotGit, err)
		return "", false
	}
	gitDir := dotGit
	if !info.IsDir() {
		data, err := g.ReadFile(dotGit)
		if err != nil {
			logReadError(g.logger(), dotGit, err)
			return "", false
		}
		line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
		target, ok := strings.CutPrefix(line, "gitdir:")
		target = strings.TrimSpace(target)
		if !ok || target == "" {
			g.logger().Debug("Ignoring .git file without a gitdir line", "path", dotGit)
			return "", false
		}
		if gitDir, ok = g.Resolve(repo, target); !ok {
			g.logger().Debug("Keeping .git file whose gitdir cannot be followed as a repository root", "path", dotGit)
			return "", true
		}
	}
	if !isGitDir(g, gitDir) {
		g.logger().Debug("Ignoring .git that is not a valid git directory", "path", dotGit, "gitdir", gitDir)
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

// commonDir returns the directory shared by all worktrees of gitDir,
// reporting false when its commondir file points where g cannot reach.
func commonDir(g gitFS, gitDir string) (string, bool) {
	p := g.Join(gitDir, "commondir")
	data, err := g.ReadFile(p)
	if err != nil {
		logReadError(g.logger(), p, err)
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
