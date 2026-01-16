package lx

import (
	"context"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type WalkerOptions struct {
	FS             fs.FS
	FollowSymlinks bool
	ShowHidden     bool
	IgnoreEnabled  bool
	GlobalIgnore   gitignore.IgnoreMatcher
	Includes       []string
	Excludes       []string
	OnIgnore       func(path string, reason string, source string)
}

type Walker struct {
	opts WalkerOptions
}

// ignoreSource pairs a matcher with the file it was loaded from.
type ignoreSource struct {
	matcher gitignore.IgnoreMatcher
	source  string
}

func NewWalker(opts WalkerOptions) *Walker {
	return &Walker{opts: opts}
}

func (w *Walker) Walk(ctx context.Context, roots []string) <-chan InputFile {
	out := make(chan InputFile)
	filesystem := w.opts.FS
	if filesystem == nil {
		filesystem = os.DirFS(".")
	}

	go func() {
		defer close(out)
		for _, root := range roots {
			cleanRoot := path.Clean(root)
			if cleanRoot == "." || cleanRoot == "/" {
				cleanRoot = "."
			}

			// Initialize ignore stack with global ignores at the root "."
			ignoreStacks := make(map[string][]ignoreSource)
			if w.opts.GlobalIgnore != nil {
				ignoreStacks["."] = []ignoreSource{{matcher: w.opts.GlobalIgnore, source: "global"}}
			}

			err := fs.WalkDir(filesystem, cleanRoot, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					out <- InputFile{Path: p, LoadError: err}
					return nil
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if !w.opts.ShowHidden && isHidden(p) {
					if w.opts.OnIgnore != nil {
						w.opts.OnIgnore(p, "hidden", "")
					}
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}

				if w.opts.IgnoreEnabled {
					parent := path.Dir(p)
					if parent == "." || parent == "/" {
						parent = "."
					}
					if p == "." {
						parent = ""
					}

					var currentStack []ignoreSource
					if p == "." {
						currentStack = ignoreStacks["."]
					} else {
						currentStack = ignoreStacks[parent]
					}

					if d.IsDir() {
						local := loadLocalIgnores(filesystem, p)
						newStack := append([]ignoreSource{}, currentStack...)
						newStack = append(newStack, local...)
						ignoreStacks[p] = newStack
					}

					ignored, source := isIgnored(currentStack, p, d.IsDir())
					if ignored && p != "." {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, "ignore-file", source)
						}
						if d.IsDir() {
							return fs.SkipDir
						}
						return nil
					}
				}

				if !d.IsDir() {
					if p == "." {
						return nil
					}

					if !IsKept(p, w.opts.Includes, w.opts.Excludes) {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, "filter-pattern", "")
						}
						return nil
					}

					info, _ := d.Info()
					out <- NewInputFile(filesystem, p, info)
				}
				return nil
			})

			if err != nil && err != context.Canceled {
				out <- InputFile{Path: root, LoadError: err}
			}
		}
	}()
	return out
}

func loadLocalIgnores(fsys fs.FS, dir string) []ignoreSource {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var sources []ignoreSource
	for _, name := range names {
		target := path.Join(dir, name)
		if data, err := fs.ReadFile(fsys, target); err == nil {
			m := gitignore.NewGitIgnoreFromReader(dir, strings.NewReader(string(data)))
			sources = append(sources, ignoreSource{matcher: m, source: target})
		}
	}
	return sources
}

func isIgnored(stack []ignoreSource, p string, isDir bool) (bool, string) {
	for _, s := range stack {
		if s.matcher != nil && s.matcher.Match(p, isDir) {
			return true, s.source
		}
	}
	return false, ""
}

func isHidden(p string) bool {
	if p == "." || p == "" {
		return false
	}
	base := path.Base(p)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
