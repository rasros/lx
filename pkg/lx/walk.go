package lx

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type WalkerOptions struct {
	FollowSymlinks bool
	ShowHidden     bool
	IgnoreEnabled  bool
	GlobalIgnore   gitignore.IgnoreMatcher
	// Hook for callers to react to ignored files (logging, stats, etc.)
	OnIgnore func(path string, reason string)
}

type Walker struct {
	Opts WalkerOptions
}

func NewWalker(opts WalkerOptions) *Walker {
	return &Walker{Opts: opts}
}

func (w *Walker) Walk(ctx context.Context, roots []string) <-chan InputFile {
	out := make(chan InputFile)
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
			var stack []gitignore.IgnoreMatcher
			if w.Opts.GlobalIgnore != nil {
				stack = append(stack, w.Opts.GlobalIgnore)
			}

			w.walkDir(ctx, root, absRoot, info, stack, visited, out)
		}
	}()
	return out
}

func (w *Walker) walkDir(ctx context.Context, path string, absPath string, info os.FileInfo, ignoreStack []gitignore.IgnoreMatcher, visited map[string]bool, out chan<- InputFile) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	if w.Opts.FollowSymlinks {
		if visited[absPath] {
			return
		}
		visited[absPath] = true
	}

	if w.Opts.IgnoreEnabled && isIgnored(ignoreStack, path, true) {
		if w.Opts.OnIgnore != nil {
			w.Opts.OnIgnore(path, "gitignore")
		}
		return
	}

	if !w.Opts.ShowHidden && isHidden(path) {
		return
	}

	newStack := ignoreStack
	if w.Opts.IgnoreEnabled {
		matchers := loadLocalIgnores(path)
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
			if !w.Opts.FollowSymlinks {
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
			abs, _ := filepath.Abs(childPath)
			targetAbs = abs
		}

		if info.IsDir() {
			if !w.Opts.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			w.walkDir(ctx, childPath, targetAbs, info, newStack, visited, out)
		} else {
			if !w.Opts.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			if w.Opts.IgnoreEnabled && isIgnored(newStack, childPath, false) {
				if w.Opts.OnIgnore != nil {
					w.Opts.OnIgnore(childPath, "gitignore")
				}
				continue
			}

			out <- NewOsInputFile(childPath, targetAbs, info)
		}
	}
}

func loadLocalIgnores(dir string) []gitignore.IgnoreMatcher {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var matchers []gitignore.IgnoreMatcher
	for _, name := range names {
		path := filepath.Join(dir, name)
		if m, err := gitignore.NewGitIgnore(path); err == nil {
			matchers = append(matchers, m)
		}
	}
	return matchers
}

func isIgnored(stack []gitignore.IgnoreMatcher, path string, isDir bool) bool {
	for _, m := range stack {
		if m != nil && m.Match(path, isDir) {
			return true
		}
	}
	return false
}

func isHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
