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

// IgnoreReason provides a typed explanation for why a file was ignored.
type IgnoreReason int

const (
	ReasonHidden IgnoreReason = iota
	ReasonIgnoreFile
	ReasonSymlinkDirSkipped
	ReasonSymlinkFileSkipped
	ReasonSymlinkCycle
	ReasonFilterPattern
)

func (ir IgnoreReason) String() string {
	switch ir {
	case ReasonHidden:
		return "hidden"
	case ReasonIgnoreFile:
		return "ignore-file"
	case ReasonSymlinkDirSkipped:
		return "symlink-dir-skipped"
	case ReasonSymlinkFileSkipped:
		return "symlink-file-skipped"
	case ReasonSymlinkCycle:
		return "symlink-cycle"
	case ReasonFilterPattern:
		return "filter-pattern"
	default:
		return "unknown"
	}
}

// WalkerOptions configures the behavior of the file system walker.
type WalkerOptions struct {
	FS                 fs.FS
	Root               string
	IgnoreFileSymlinks bool
	IgnoreDirSymlinks  bool
	IgnoreHidden       bool
	IgnoreEnabled      bool
	GlobalIgnore       gitignore.IgnoreMatcher
	Includes           []string
	Excludes           []string
	OnIgnore           func(path string, reason IgnoreReason, source string)
	OnIgnoreFileLoaded func(path string, isAncestor bool)
}

// Walker encapsulates the logic for traversing a file system with filtering and ignore rules.
type Walker struct {
	opts WalkerOptions
}

type ignoreSource struct {
	matcher gitignore.IgnoreMatcher
	source  string
}

func NewWalker(opts WalkerOptions) *Walker {
	return &Walker{opts: opts}
}

// Walk starts a goroutine to traverse the roots and send discovered files to the returned channel.
func (w *Walker) Walk(ctx context.Context, roots []string) <-chan InputFile {
	out := make(chan InputFile)
	filesystem := w.opts.FS
	if filesystem == nil {
		filesystem = os.DirFS(".")
	}

	go func() {
		defer close(out)

		visitedDirs := make(map[string]struct{})
		var walkFn fs.WalkDirFunc

		ignoreStacks := make(map[string][]ignoreSource)

		// 1. Initialize Global Ignores
		if w.opts.GlobalIgnore != nil {
			ignoreStacks["."] = []ignoreSource{{matcher: w.opts.GlobalIgnore, source: "global"}}
		} else {
			ignoreStacks["."] = []ignoreSource{}
		}

		// 2. Load Ancestor Ignores (e.g. project root .gitignore when running in /src)
		// We can only do this reliably if we are running against the OS filesystem (cwd).
		if cwd, err := os.Getwd(); err == nil && w.opts.IgnoreEnabled {
			ancestors := loadAncestorIgnores(cwd, w.opts.OnIgnoreFileLoaded)
			// Append ancestors to the base stack.
			ignoreStacks["."] = append(ignoreStacks["."], ancestors...)
		}

		// 3. Pre-load ignore stacks for parents of the requested roots.
		for _, root := range roots {
			cleanRoot := path.Clean(root)
			if cleanRoot == "." || cleanRoot == "/" {
				continue
			}

			// We need to ensure the stack exists for the *parent* of the root.
			dir := path.Dir(cleanRoot)
			if dir == "." || dir == "/" {
				continue
			}

			parts := strings.Split(dir, "/")
			currentPath := ""

			for _, part := range parts {
				parent := currentPath
				if parent == "" {
					parent = "."
				}

				if currentPath == "" {
					currentPath = part
				} else {
					currentPath = path.Join(currentPath, part)
				}

				if _, exists := ignoreStacks[currentPath]; !exists {
					// Inherit from parent
					parentStack, ok := ignoreStacks[parent]
					if !ok {
						if w.opts.GlobalIgnore != nil {
							parentStack = []ignoreSource{{matcher: w.opts.GlobalIgnore, source: "global"}}
						}
					}

					local := loadLocalIgnores(filesystem, currentPath, w.opts.OnIgnoreFileLoaded)
					newStack := append([]ignoreSource{}, parentStack...)
					newStack = append(newStack, local...)
					ignoreStacks[currentPath] = newStack
				}
			}
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
					w.opts.OnIgnore(p, ReasonHidden, "")
				}
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			isSymlink := (d.Type() & fs.ModeSymlink) != 0

			if isSymlink {
				info, err := fs.Stat(filesystem, p)

				isTargetDir := (err == nil && info.IsDir())

				if isTargetDir {
					if w.opts.IgnoreDirSymlinks {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, ReasonSymlinkDirSkipped, "")
						}
						return nil
					}

					absPath := p
					if w.opts.Root != "" {
						absPath = filepath.Join(w.opts.Root, p)
					} else {
						if abs, err := filepath.Abs(p); err == nil {
							absPath = abs
						}
					}
					realPath, err := filepath.EvalSymlinks(absPath)
					if err == nil {
						if _, exists := visitedDirs[realPath]; exists {
							if w.opts.OnIgnore != nil {
								w.opts.OnIgnore(p, ReasonSymlinkCycle, "")
							}
							return nil
						}
						visitedDirs[realPath] = struct{}{}
					}
					return fs.WalkDir(filesystem, p, walkFn)

				} else {
					if w.opts.IgnoreFileSymlinks {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(p, ReasonSymlinkFileSkipped, "")
						}
						return nil
					}
				}
			}

			if d.IsDir() && !isSymlink {
				absPath := p
				if w.opts.Root != "" {
					absPath = filepath.Join(w.opts.Root, p)
				} else {
					if abs, err := filepath.Abs(p); err == nil {
						absPath = abs
					}
				}
				if realPath, err := filepath.EvalSymlinks(absPath); err == nil {
					visitedDirs[realPath] = struct{}{}
				}
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
					local := loadLocalIgnores(filesystem, p, w.opts.OnIgnoreFileLoaded)
					newStack := append([]ignoreSource{}, currentStack...)
					newStack = append(newStack, local...)
					ignoreStacks[p] = newStack
				}

				ignored, source := isIgnored(currentStack, p, d.IsDir())
				if ignored && p != "." {
					if w.opts.OnIgnore != nil {
						w.opts.OnIgnore(p, ReasonIgnoreFile, source)
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
						w.opts.OnIgnore(p, ReasonFilterPattern, "")
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

func loadLocalIgnores(fsys fs.FS, dir string, onLoad func(path string, isAncestor bool)) []ignoreSource {
	names := []string{".gitignore", ".ignore", ".lxignore"}
	var sources []ignoreSource
	for _, name := range names {
		target := path.Join(dir, name)
		if data, err := fs.ReadFile(fsys, target); err == nil {
			if onLoad != nil {
				onLoad(target, false)
			}
			m := gitignore.NewGitIgnoreFromReader(dir, strings.NewReader(string(data)))
			sources = append(sources, ignoreSource{matcher: m, source: target})
		}
	}
	return sources
}

// loadAncestorIgnores climbs the directory tree from cwd to root/home
// and loads any ignore files found.
func loadAncestorIgnores(cwd string, onLoad func(path string, isAncestor bool)) []ignoreSource {
	var sources []ignoreSource

	var pathStack []string
	curr := filepath.Dir(cwd) // Start from parent

	for {
		if curr == "" || curr == "." || curr == "/" || curr == filepath.Dir(curr) {
			break
		}
		pathStack = append(pathStack, curr)
		curr = filepath.Dir(curr)
	}

	// Iterate from Root down to Parent
	for i := len(pathStack) - 1; i >= 0; i-- {
		dir := pathStack[i]

		names := []string{".gitignore", ".ignore", ".lxignore"}
		for _, name := range names {
			target := filepath.Join(dir, name)
			if data, err := os.ReadFile(target); err == nil {
				if onLoad != nil {
					onLoad(target, true)
				}
				m := gitignore.NewGitIgnoreFromReader(".", strings.NewReader(string(data)))
				sources = append(sources, ignoreSource{matcher: m, source: target})
			}
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
