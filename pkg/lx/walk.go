package lx

import (
	"bytes"
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

// Walk accepts a list of root paths and returns a channel of InputFiles.
// It handles directory traversal, ignores, and eventually archives.
func (w *Walker) Walk(roots []string) <-chan InputFile {
	out := make(chan InputFile)

	globalIgnore := w.loadGlobalIgnores()

	go func() {
		defer close(out)

		visited := make(map[string]bool)

		for _, root := range roots {
			if root == "-" {
				continue
			}

			info, err := os.Stat(root)
			if err != nil {
				out <- InputFile{Path: root, LoadError: err}
				continue
			}

			if !info.IsDir() {
				if abs, err := filepath.Abs(root); err == nil {
					out <- NewOsInputFile(root, abs, info)
				}
				continue
			}

			absRoot, _ := filepath.Abs(root)
			w.walkDir(root, absRoot, info, []gitignore.IgnoreMatcher{globalIgnore}, visited, out)
		}
	}()

	return out
}

func (w *Walker) walkDir(
	path string,
	absPath string,
	info os.FileInfo,
	ignoreStack []gitignore.IgnoreMatcher,
	visited map[string]bool,
	out chan<- InputFile,
) {
	if w.Config.FollowSymlinks {
		if visited[absPath] {
			return
		}
		visited[absPath] = true
	}

	if !w.Config.NoIgnore && isIgnored(ignoreStack, path, true) {
		return
	}

	if !w.Config.ShowHidden && isHidden(path) {
		return
	}

	newStack := ignoreStack
	if !w.Config.NoIgnore {
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
		out <- InputFile{Path: path, LoadError: err}
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
			w.walkDir(childPath, targetAbs, info, newStack, visited, out)
		} else {
			if !w.Config.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			if !w.Config.NoIgnore && isIgnored(newStack, childPath, false) {
				continue
			}

			// TODO: Archive Handling Check (Zip, etc) goes here later

			out <- NewOsInputFile(childPath, targetAbs, info)
		}
	}
}

func (w *Walker) loadGlobalIgnores() gitignore.IgnoreMatcher {
	if w.Config.NoIgnore {
		return nil
	}

	var lines []string

	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()

	candidates := []string{
		filepath.Join(configDir, "git", "ignore"),
		filepath.Join(configDir, "lx", "ignore"),
	}

	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "git", "ignore"))
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
