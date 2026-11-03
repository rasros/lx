package walker

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
)

// parseRuleFile parses an ignore file. It splits on newlines rather than
// scanning, so no line length limit can drop the rules after a long line.
func parseRuleFile(data []byte, basePath, source string) []Rule {
	return parseRules(strings.Split(string(data), "\n"), basePath, source)
}

// logReadError logs why an existing rule file could not be read.
func logReadError(log *slog.Logger, path string, err error) {
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Debug("Failed to read ignore rules", "path", path, "error", err)
	}
}

// findGitDir reports whether repo, a directory on the host, is a repository
// root, and returns its git directory: a .git directory holding a HEAD, or
// the directory a .git file names on its "gitdir:" line, as linked worktrees
// and submodules have it. Anything else named .git is no repository.
func findGitDir(h Host, repo string) (string, bool) {
	dotGit := filepath.Join(repo, ".git")
	info, err := h.Stat(dotGit)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		_, err := h.Stat(filepath.Join(dotGit, "HEAD"))
		return dotGit, err == nil
	}
	data, err := h.ReadFile(dotGit)
	if err != nil {
		return "", false
	}
	line, _, _ := strings.Cut(string(data), "\n")
	target, ok := strings.CutPrefix(strings.TrimSpace(line), "gitdir:")
	if target = strings.TrimSpace(target); !ok || target == "" {
		return "", false
	}
	return resolveRelative(repo, target), true
}

// resolveRelative resolves target, a path read from a .git or commondir
// file, against base.
func resolveRelative(base, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(base, target)
}
