package lx

import (
	"context"
	"io"
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
	OnIgnore       func(path string, reason string)
}

type Walker struct {
	opts WalkerOptions
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
			ignoreStacks := make(map[string][]gitignore.IgnoreMatcher)
			if w.opts.GlobalIgnore != nil {
				ignoreStacks["."] = []gitignore.IgnoreMatcher{w.opts.GlobalIgnore}
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

				// 1. Hidden Files Check
				if !w.opts.ShowHidden && isHidden(p) {
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}

				// 2. Ignore Logic
				if w.opts.IgnoreEnabled {
					// Identify parent to inherit ignore stack
					parent := path.Dir(p)
					if parent == "." || parent == "/" {
						parent = "."
					}
					if p == "." {
						parent = "" // Root has no parent stack, it starts fresh or uses global
					}

					// Inherit from parent
					var currentStack []gitignore.IgnoreMatcher
					if p == "." {
						currentStack = ignoreStacks["."]
					} else {
						currentStack = ignoreStacks[parent]
					}

					// If directory, load local ignores and store for children
					if d.IsDir() {
						local := loadLocalIgnores(filesystem, p)
						newStack := append([]gitignore.IgnoreMatcher{}, currentStack...)
						newStack = append(newStack, local...)
						ignoreStacks[p] = newStack
					}

					// Check if current path is ignored
					if isIgnored(currentStack, p, d.IsDir()) && p != "." {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, "gitignore")
						}
						if d.IsDir() {
							return fs.SkipDir
						}
						return nil
					}
				}

				// 3. File Processing (apply user filters)
				if !d.IsDir() {
					// Skip the root "." if it somehow appears as a file (unlikely)
					if p == "." {
						return nil
					}

					if !IsKept(p, w.opts.Includes, w.opts.Excludes) {
						return nil
					}

					info, _ := d.Info()
					out <- InputFile{
						Path:    p,
						Size:    info.Size(),
						ModTime: info.ModTime(),
						Open: func() (io.ReadCloser, error) {
							return filesystem.Open(p)
						},
					}
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

func loadLocalIgnores(fsys fs.FS, dir string) []gitignore.IgnoreMatcher {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var matchers []gitignore.IgnoreMatcher
	for _, name := range names {
		target := path.Join(dir, name)
		if data, err := fs.ReadFile(fsys, target); err == nil {
			matchers = append(matchers, gitignore.NewGitIgnoreFromReader(dir, strings.NewReader(string(data))))
		}
	}
	return matchers
}

func isIgnored(stack []gitignore.IgnoreMatcher, p string, isDir bool) bool {
	for _, m := range stack {
		if m != nil && m.Match(p, isDir) {
			return true
		}
	}
	return false
}

func isHidden(p string) bool {
	if p == "." || p == "" {
		return false
	}
	base := path.Base(p)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}
