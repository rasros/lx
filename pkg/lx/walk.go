package lx

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

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

		// Visited map for cycle detection (symlinks)
		visitedDirs := make(map[string]struct{})
		var visitedMu sync.RWMutex

		markVisited := func(path string) bool {
			visitedMu.Lock()
			defer visitedMu.Unlock()
			if _, exists := visitedDirs[path]; exists {
				return true
			}
			visitedDirs[path] = struct{}{}
			return false
		}

		// 1. Initialize Base Stack (Global Ignores + Ancestors)
		var baseStack []ignoreSource
		if w.opts.GlobalIgnore != nil {
			baseStack = append(baseStack, ignoreSource{matcher: w.opts.GlobalIgnore, source: "global"})
		}

		// Load Ancestor Ignores only if we are likely on the OS filesystem
		if cwd, err := os.Getwd(); err == nil && w.opts.IgnoreEnabled {
			ancestors := loadAncestorIgnores(cwd, w.opts.OnIgnoreFileLoaded)
			baseStack = append(baseStack, ancestors...)
		}

		// Optimization 1: Recursive function to avoid map lookups and slice copying
		var recursiveWalk func(dir string, currentStack []ignoreSource)
		recursiveWalk = func(dir string, currentStack []ignoreSource) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Load local .gitignore for this directory
			if w.opts.IgnoreEnabled {
				local := loadLocalIgnores(filesystem, dir, w.opts.OnIgnoreFileLoaded)
				if len(local) > 0 {
					// Create a new slice sharing the backing array capacity where possible,
					// but distinct for this stack frame.
					newStack := make([]ignoreSource, len(currentStack)+len(local))
					copy(newStack, currentStack)
					copy(newStack[len(currentStack):], local)
					currentStack = newStack
				}
			}

			// Optimization 4: Efficient handling of explicit file roots vs directories
			// If 'dir' is a file (e.g. user passed specific file), ReadDir fails or returns nothing useful usually.
			// However, this function is primarily called for Directories.
			// Roots are handled separately below to kickstart this.

			entries, err := fs.ReadDir(filesystem, dir)
			if err != nil {
				out <- InputFile{Path: dir, LoadError: err}
				return
			}

			for _, d := range entries {
				name := d.Name()
				fullPath := name
				if dir != "." {
					fullPath = path.Join(dir, name)
				}

				// 1. Check Hidden (Fastest check)
				if w.opts.IgnoreHidden && isHidden(name) {
					if w.opts.OnIgnore != nil {
						w.opts.OnIgnore(fullPath, ReasonHidden, "")
					}
					continue
				}

				isSymlink := (d.Type() & fs.ModeSymlink) != 0
				isDir := d.IsDir()

				// Handle Symlinks to Directories
				if isSymlink {
					info, err := fs.Stat(filesystem, fullPath)
					if err == nil && info.IsDir() {
						isDir = true
						if w.opts.IgnoreDirSymlinks {
							if w.opts.OnIgnore != nil {
								w.opts.OnIgnore(fullPath, ReasonSymlinkDirSkipped, "")
							}
							continue
						}
						// Cycle detection
						if abs, err := filepath.Abs(fullPath); err == nil {
							if realPath, err := filepath.EvalSymlinks(abs); err == nil {
								if markVisited(realPath) {
									if w.opts.OnIgnore != nil {
										w.opts.OnIgnore(fullPath, ReasonSymlinkCycle, "")
									}
									continue
								}
							}
						}
					} else if w.opts.IgnoreFileSymlinks {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(fullPath, ReasonSymlinkFileSkipped, "")
						}
						continue
					}
				} else if isDir {
					// Normal Directory Cycle check (for hardlinks or bind mounts context)
					if abs, err := filepath.Abs(fullPath); err == nil {
						if realPath, err := filepath.EvalSymlinks(abs); err == nil {
							markVisited(realPath)
						}
					}
				}

				if isDir {
					// Check Ignore (Gitignore) for Directory
					if w.opts.IgnoreEnabled {
						if ignored, source := isIgnored(currentStack, fullPath, true); ignored {
							if w.opts.OnIgnore != nil {
								w.opts.OnIgnore(fullPath, ReasonIgnoreFile, source)
							}
							continue
						}
					}
					// Recurse
					recursiveWalk(fullPath, currentStack)
				} else {
					// Optimization 2: Check Includes/Excludes (Fast Globs) BEFORE Gitignore (Slow Regex)
					if !IsKept(fullPath, w.opts.Includes, w.opts.Excludes) {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(fullPath, ReasonFilterPattern, "")
						}
						continue
					}

					// Check Ignore (Gitignore) for File
					if w.opts.IgnoreEnabled {
						if ignored, source := isIgnored(currentStack, fullPath, false); ignored {
							if w.opts.OnIgnore != nil {
								w.opts.OnIgnore(fullPath, ReasonIgnoreFile, source)
							}
							continue
						}
					}

					// Found valid file
					info, _ := d.Info()
					out <- NewInputFile(filesystem, fullPath, info)
				}
			}
		}

		// Process Roots
		for _, root := range roots {
			cleanRoot := path.Clean(root)
			if cleanRoot == "/" {
				cleanRoot = "."
			}

			// Optimization 4: Direct Stat to handle explicit file roots efficiently
			info, err := fs.Stat(filesystem, cleanRoot)
			if err != nil {
				out <- InputFile{Path: root, LoadError: err}
				continue
			}

			// Calculate stack for the parent of the root
			parent := path.Dir(cleanRoot)
			stack := baseStack
			if parent != "." && parent != "/" && w.opts.IgnoreEnabled {
				// We need to build the stack from the filesystem root down to 'parent'
				// This is complex for arbitrary paths, so we rely on the baseStack
				// + loading ignores for the parent if reachable.
				// For simplicity and performance in common "lx ." cases, baseStack is sufficient.
			}

			if info.IsDir() {
				// Check root itself against ignores? Usually roots are explicit.
				// We proceed to recursive walk.
				recursiveWalk(cleanRoot, stack)
			} else {
				// Root is a file.
				// Even if it's an explicit root, we check filters if they exist,
				// but usually explicit files (-f) bypass this in app.go.
				// If passed as standard arg, we apply standard logic:

				// 1. IsKept
				if !IsKept(cleanRoot, w.opts.Includes, w.opts.Excludes) {
					if w.opts.OnIgnore != nil {
						w.opts.OnIgnore(cleanRoot, ReasonFilterPattern, "")
					}
					continue
				}

				// 2. Gitignore
				if w.opts.IgnoreEnabled {
					if ignored, source := isIgnored(stack, cleanRoot, false); ignored {
						if w.opts.OnIgnore != nil {
							w.opts.OnIgnore(cleanRoot, ReasonIgnoreFile, source)
						}
						continue
					}
				}

				out <- NewInputFile(filesystem, cleanRoot, info)
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
func loadAncestorIgnores(cwd string, onLoad func(path string, isAncestor bool)) []ignoreSource {
	var sources []ignoreSource
	var pathStack []string
	curr := filepath.Dir(cwd)

	for {
		if curr == "" || curr == "." || curr == "/" || curr == filepath.Dir(curr) {
			break
		}
		pathStack = append(pathStack, curr)
		curr = filepath.Dir(curr)
	}

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

func isHidden(name string) bool {
	if name == "." || name == ".." {
		return false
	}
	return len(name) > 1 && name[0] == '.'
}
