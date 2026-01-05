package lx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type Walker struct {
	Config Config
}

func NewWalker(cfg Config) *Walker {
	return &Walker{Config: cfg}
}

// Walk accepts a context and a list of root paths and returns a channel of InputFiles.
// It handles directory traversal, ignores, and eventually archives.
func (w *Walker) Walk(ctx context.Context, roots []string) <-chan InputFile {
	out := make(chan InputFile)

	globalIgnore := w.loadGlobalIgnores()

	go func() {
		defer close(out)

		visited := make(map[string]bool)

		for _, root := range roots {
			// Check for cancellation before processing next root
			select {
			case <-ctx.Done():
				return
			default:
			}

			if root == "-" {
				continue
			}

			info, err := os.Stat(root)
			if err != nil {
				select {
				case out <- InputFile{Path: root, LoadError: err}:
				case <-ctx.Done():
					return
				}
				continue
			}

			if !info.IsDir() {
				if abs, err := filepath.Abs(root); err == nil {
					select {
					case out <- NewOsInputFile(root, abs, info):
					case <-ctx.Done():
						return
					}
				}
				continue
			}

			absRoot, _ := filepath.Abs(root)
			w.walkDir(ctx, root, absRoot, info, []gitignore.IgnoreMatcher{globalIgnore}, visited, out)
		}
	}()

	return out
}

func (w *Walker) walkDir(
	ctx context.Context,
	path string,
	absPath string,
	info os.FileInfo,
	ignoreStack []gitignore.IgnoreMatcher,
	visited map[string]bool,
	out chan<- InputFile,
) {
	// Check for cancellation at the start of directory processing
	select {
	case <-ctx.Done():
		return
	default:
	}

	if w.Config.FollowSymlinks {
		if visited[absPath] {
			return
		}
		visited[absPath] = true
	}

	// Respect ignore files if enabled
	if w.Config.IgnoreEnabled() && isIgnored(ignoreStack, path, true) {
		return
	}

	if !w.Config.ShowHidden && isHidden(path) {
		return
	}

	newStack := ignoreStack
	if w.Config.IgnoreEnabled() {
		matchers := w.loadLocalIgnores(path)
		if len(matchers) > 0 {
			ns := make([]gitignore.IgnoreMatcher, len(ignoreStack)+len(matchers))
			copy(ns, ignoreStack)
			copy(ns[len(ignoreStack):], matchers)
			newStack = ns
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		select {
		case out <- InputFile{Path: path, LoadError: err}:
		case <-ctx.Done():
			return
		}
		return
	}

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		isSymlink := (info.Mode() & os.ModeSymlink) != 0
		var targetAbs string

		if isSymlink {
			if !w.Config.FollowSymlinks {
				continue
			}
			resolved, err := filepath.EvalSymlinks(childPath)
			if err != nil {
				continue
			}
			targetInfo, err := os.Stat(resolved)
			if err != nil {
				continue
			}
			info = targetInfo
			targetAbs = resolved
		} else {
			if abs, err := filepath.Abs(childPath); err == nil {
				targetAbs = abs
			} else {
				targetAbs = childPath
			}
		}

		if info.IsDir() {
			if !w.Config.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			w.walkDir(ctx, childPath, targetAbs, info, newStack, visited, out)
		} else {
			if !w.Config.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			if w.Config.IgnoreEnabled() && isIgnored(newStack, childPath, false) {
				continue
			}

			// TODO: Archive Handling Check (Zip, etc) goes here later

			select {
			case out <- NewOsInputFile(childPath, targetAbs, info):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Walker) loadGlobalIgnores() gitignore.IgnoreMatcher {
	if !w.Config.IgnoreEnabled() {
		return nil
	}

	var lines []string

	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()

	candidates := []string{
		filepath.Join(configDir, "lx", "ignore"),
	}

	// XDG Support for global gitignore
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	if xdgConfig != "" {
		candidates = append(candidates, filepath.Join(xdgConfig, "git", "ignore"))
	}

	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			lines = append(lines, strings.Split(string(data), "\n")...)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	buf := bytes.NewBufferString(strings.Join(lines, "\n"))
	return gitignore.NewGitIgnoreFromReader(".", buf)
}

func (w *Walker) loadLocalIgnores(dir string) []gitignore.IgnoreMatcher {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var matchers []gitignore.IgnoreMatcher

	for _, name := range names {
		path := filepath.Join(dir, name)
		if m, err := gitignore.NewGitIgnore(path); err == nil {
			matchers = append(matchers, m)
		}
	}

	gitInfo := filepath.Join(dir, ".git", "info", "exclude")
	if m, err := gitignore.NewGitIgnore(gitInfo); err == nil {
		matchers = append(matchers, m)
	}

	return matchers
}

func isIgnored(stack []gitignore.IgnoreMatcher, path string, isDir bool) bool {
	for _, m := range stack {
		if m == nil {
			continue
		}
		if m.Match(path, isDir) {
			return true
		}
	}
	return false
}

func isHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
