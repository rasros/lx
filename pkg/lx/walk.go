package lx

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type Walker struct {
	Config Config
}

func NewWalker(cfg Config) *Walker {
	cfg.EnsureLogger()
	return &Walker{Config: cfg}
}

// Walk accepts a context and a list of root paths and returns a channel of InputFiles.
func (w *Walker) Walk(ctx context.Context, roots []string) <-chan InputFile {
	out := make(chan InputFile)
	log := w.Config.Logger

	go func() {
		defer close(out)

		visited := make(map[string]bool)

		for _, root := range roots {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if root == "-" {
				continue
			}

			log.Debug("walking root", "path", root)

			info, err := os.Stat(root)
			if err != nil {
				log.Warn("failed to stat root", "path", root, "error", err)
				select {
				case out <- InputFile{Path: root, LoadError: err}:
				case <-ctx.Done():
					return
				}
				continue
			}

			if !info.IsDir() {
				// Trace level log
				log.Log(ctx, slog.LevelDebug-1, "root is file", "path", root)

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

			// Initialize ignore stack with GlobalIgnore if it exists
			var stack []gitignore.IgnoreMatcher
			if w.Config.GlobalIgnore != nil {
				stack = append(stack, w.Config.GlobalIgnore)
			}

			w.walkDir(ctx, root, absRoot, info, stack, visited, out)
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
	log := w.Config.Logger

	// Trace level
	log.Log(ctx, slog.LevelDebug-1, "entering dir", "path", path)

	select {
	case <-ctx.Done():
		return
	default:
	}

	if w.Config.FollowSymlinks {
		if visited[absPath] {
			log.Info("cycle detected, skipping", "path", path)
			return
		}
		visited[absPath] = true
	}

	if w.Config.IgnoreEnabled() && isIgnored(ignoreStack, path, true) {
		log.Debug("ignored directory", "path", path)
		return
	}

	if !w.Config.ShowHidden && isHidden(path) {
		// Trace
		log.Log(ctx, slog.LevelDebug-1, "skipping hidden directory", "path", path)
		return
	}

	newStack := ignoreStack
	if w.Config.IgnoreEnabled() {
		matchers := w.loadLocalIgnores(path)
		if len(matchers) > 0 {
			// Trace
			log.Log(ctx, slog.LevelDebug-1, "applying local ignore files", "count", len(matchers), "path", path)

			ns := make([]gitignore.IgnoreMatcher, len(ignoreStack)+len(matchers))
			copy(ns, ignoreStack)
			copy(ns[len(ignoreStack):], matchers)
			newStack = ns
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Error("read dir failed", "path", path, "error", err)
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
			log.Warn("stat failed", "path", childPath, "error", err)
			continue
		}

		isSymlink := (info.Mode() & os.ModeSymlink) != 0
		var targetAbs string

		if isSymlink {
			if !w.Config.FollowSymlinks {
				// Trace
				log.Log(ctx, slog.LevelDebug-1, "skipping symlink (follow disabled)", "path", childPath)
				continue
			}
			resolved, err := filepath.EvalSymlinks(childPath)
			if err != nil {
				log.Warn("broken symlink", "path", childPath, "error", err)
				continue
			}
			targetInfo, err := os.Stat(resolved)
			if err != nil {
				continue
			}
			info = targetInfo
			targetAbs = resolved
			// Trace
			log.Log(ctx, slog.LevelDebug-1, "followed symlink", "path", childPath, "target", resolved)
		} else {
			if abs, err := filepath.Abs(childPath); err == nil {
				targetAbs = abs
			} else {
				targetAbs = childPath
			}
		}

		if info.IsDir() {
			if !w.Config.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				// Trace
				log.Log(ctx, slog.LevelDebug-1, "skipping hidden dir", "path", childPath)
				continue
			}
			w.walkDir(ctx, childPath, targetAbs, info, newStack, visited, out)
		} else {
			if !w.Config.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				// Trace
				log.Log(ctx, slog.LevelDebug-1, "skipping hidden file", "path", childPath)
				continue
			}

			if w.Config.IgnoreEnabled() && isIgnored(newStack, childPath, false) {
				// Trace
				log.Log(ctx, slog.LevelDebug-1, "ignored file", "path", childPath)
				continue
			}

			// Trace
			log.Log(ctx, slog.LevelDebug-1, "found file", "path", childPath)

			select {
			case out <- NewOsInputFile(childPath, targetAbs, info):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Walker) loadLocalIgnores(dir string) []gitignore.IgnoreMatcher {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var matchers []gitignore.IgnoreMatcher
	log := w.Config.Logger

	for _, name := range names {
		path := filepath.Join(dir, name)
		if m, err := gitignore.NewGitIgnore(path); err == nil {
			// Trace
			log.Log(context.Background(), slog.LevelDebug-1, "loaded ignore file", "path", path)
			matchers = append(matchers, m)
		}
	}

	gitInfo := filepath.Join(dir, ".git", "info", "exclude")
	if m, err := gitignore.NewGitIgnore(gitInfo); err == nil {
		// Trace
		log.Log(context.Background(), slog.LevelDebug-1, "loaded git exclude", "path", gitInfo)
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
