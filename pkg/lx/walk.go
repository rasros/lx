package lx

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type FilterFunc func(f InputFile) bool

type WalkerOptions struct {
	FollowSymlinks bool
	ShowHidden     bool
	IgnoreEnabled  bool
	GlobalIgnore   gitignore.IgnoreMatcher
	OnIgnore       func(path string, reason string)
}

type Walker struct {
	Opts WalkerOptions
}

func NewWalker(opts WalkerOptions) *Walker {
	return &Walker{Opts: opts}
}

// Walk recursively discovers files. You can pass FilterFuncs to skip files early.
func (w *Walker) Walk(ctx context.Context, roots []string, filters ...FilterFunc) <-chan InputFile {
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

			info, err := os.Stat(root)
			if err != nil {
				out <- InputFile{Path: root, LoadError: err}
				continue
			}

			if !info.IsDir() {
				abs, _ := filepath.Abs(root)
				file := NewOsInputFile(root, abs, info)
				if w.applyFilters(file, filters) {
					out <- file
				}
				continue
			}

			absRoot, _ := filepath.Abs(root)
			var stack []gitignore.IgnoreMatcher
			if w.Opts.GlobalIgnore != nil {
				stack = append(stack, w.Opts.GlobalIgnore)
			}

			w.walkDir(ctx, root, absRoot, info, stack, visited, out, filters)
		}
	}()
	return out
}

func (w *Walker) applyFilters(f InputFile, filters []FilterFunc) bool {
	for _, filter := range filters {
		if !filter(f) {
			return false
		}
	}
	return true
}

func (w *Walker) walkDir(ctx context.Context, path string, absPath string, info os.FileInfo, ignoreStack []gitignore.IgnoreMatcher, visited map[string]bool, out chan<- InputFile, filters []FilterFunc) {
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
		if matchers := loadLocalIgnores(path); len(matchers) > 0 {
			newStack = append(append([]gitignore.IgnoreMatcher{}, ignoreStack...), matchers...)
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

		targetAbs, _ := filepath.Abs(childPath)
		if info.IsDir() {
			w.walkDir(ctx, childPath, targetAbs, info, newStack, visited, out, filters)
		} else {
			if !w.Opts.ShowHidden && isHidden(childPath) {
				continue
			}

			if w.Opts.IgnoreEnabled && isIgnored(newStack, childPath, false) {
				if w.Opts.OnIgnore != nil {
					w.Opts.OnIgnore(childPath, "gitignore")
				}
				continue
			}

			file := NewOsInputFile(childPath, targetAbs, info)
			if w.applyFilters(file, filters) {
				out <- file
			}
		}
	}
}

// Internal Helper Functions

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
