package discover

import (
	"log/slog"
	"path/filepath"

	"github.com/rasros/lx/pkg/lx/walker"
)

// selector is the include, exclude and output-path policy of one Collect,
// shared by the walk of a directory and the walks of the archives in it.
// Excludes inside a walk are the walker's override rules, which follow
// ignore-file semantics; the selector matches them only against a piped
// target, which no walker judges.
type selector struct {
	includes []walker.CompiledSpec
	excludes []walker.CompiledSpec
	outPath  string
}

func newSelector(opts Options) selector {
	return selector{
		includes: walker.CompileSpecs(opts.Includes),
		excludes: walker.CompileSpecs(opts.Excludes),
		outPath:  opts.OutPath,
	}
}

// AcceptRoot reports whether a piped target, at rel from the working
// directory, is collected. A directory is kept when an include could match
// something below it, and is then walked with the patterns applied inside.
func (s selector) AcceptRoot(rel string, isDir bool) bool {
	if len(s.excludes) > 0 && walker.IsMatchAnyCompiled(s.excludes, rel) {
		return false
	}
	if isDir {
		return walker.CouldMatchAnyDescendant(s.includes, rel)
	}
	return walker.IsMatchAnyCompiled(s.includes, rel)
}

// Descend reports whether a walk enters dir, relative to the walked root: not
// when no include could match anything below it.
func (s selector) Descend(dir string) bool {
	return walker.CouldMatchAnyDescendant(s.includes, dir)
}

// Accept reports whether the file at rel, relative to the walked root and
// displayed as path, is collected.
func (s selector) Accept(rel, path string) bool {
	if !walker.IsMatchAnyCompiled(s.includes, rel) {
		slog.Debug("Ignored by include filter (-i)", "path", path)
		return false
	}
	return !s.isOutput(path)
}

// isOutput reports whether path is the file output is written to, which is
// never collected, even when named.
func (s selector) isOutput(path string) bool {
	if s.outPath == "" {
		return false
	}
	if abs, _ := filepath.Abs(path); abs == s.outPath {
		slog.Warn("Skipping output file to avoid infinite recursion", "path", path)
		return true
	}
	return false
}
