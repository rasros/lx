package lx

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type WalkerOptions struct {
	FS             fs.FS
	Root           string // The physical root path on OS (needed for resolving symlink cycles)
	IgnoreSymlinks bool   // Replaced FollowSymlinks
	IgnoreHidden   bool   // Replaced ShowHidden
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

		// Visited map tracks canonical paths of DIRECTORIES we have entered
		// to prevent infinite recursion loops via symlinks.
		visitedDirs := make(map[string]struct{})

		var walkFn fs.WalkDirFunc

		// Maintain ignore stacks
		ignoreStacks := make(map[string][]ignoreSource)
		if w.opts.GlobalIgnore != nil {
			ignoreStacks["."] = []ignoreSource{{matcher: w.opts.GlobalIgnore, source: "global"}}
		}

		walkFn = func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				out <- InputFile{Path: p, LoadError: err}
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if w.opts.IgnoreHidden && isHidden(p) {
				if w.opts.OnIgnore != nil {
					w.opts.OnIgnore(p, "hidden", "")
				}
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			// SYMLINK HANDLING
			isSymlink := (d.Type() & fs.ModeSymlink) != 0

			// 1. If we ignore symlinks, skip them.
			if w.opts.IgnoreSymlinks && isSymlink {
				if w.opts.OnIgnore != nil {
					w.opts.OnIgnore(p, "symlink-skipped", "")
				}
				return nil
			}

			// 2. If we DO follow symlinks, check if it's a directory.
			if !w.opts.IgnoreSymlinks && isSymlink {
				info, err := fs.Stat(filesystem, p)
				if err == nil && info.IsDir() {
					// CYCLE DETECTION:
					// To safely recurse into a symlink directory, we must know its canonical path.
					// Since our FS might be virtual, we rely on the user providing opts.Root for OS resolution.
					// If opts.Root is empty, we fall back to assuming relative to CWD.

					absPath := p
					if w.opts.Root != "" {
						absPath = filepath.Join(w.opts.Root, p)
					} else {
						// Fallback: Resolve against CWD if possible
						if abs, err := filepath.Abs(p); err == nil {
							absPath = abs
						}
					}

					// Resolve the real path on disk
					realPath, err := filepath.EvalSymlinks(absPath)
					if err != nil {
						// If we can't resolve it (broken link?), treat as file to report error later
						return nil
					}

					// Check if we've been here
					if _, exists := visitedDirs[realPath]; exists {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, "symlink-cycle", "")
						}
						return nil // Skip cycle
					}

					// Mark as visited
					visitedDirs[realPath] = struct{}{}

					// Recurse manually
					return fs.WalkDir(filesystem, p, walkFn)
				}
				// File symlinks fall through
			}

			// Regular Directory Tracking for Cycles (if roots contain symlinks)
			if d.IsDir() && !isSymlink {
				// We also track regular directories to ensure if we enter them via a symlink later
				// we catch the cycle.
				absPath := p
				if w.opts.Root != "" {
					absPath = filepath.Join(w.opts.Root, p)
				} else {
					if abs, err := filepath.Abs(p); err == nil {
						absPath = abs
					}
				}

				// For regular dirs, EvalSymlinks usually just cleans the path unless parent is symlink
				if realPath, err := filepath.EvalSymlinks(absPath); err == nil {
					visitedDirs[realPath] = struct{}{}
				}
			}

			// IGNORE FILES HANDLING
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
		}

		for _, root := range roots {
			cleanRoot := path.Clean(root)
			if cleanRoot == "." || cleanRoot == "/" {
				cleanRoot = "."
			}

			err := fs.WalkDir(filesystem, cleanRoot, walkFn)

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
