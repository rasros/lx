package walker

import (
	"log/slog"
	"strings"
)

// Rule represents a single parsed line from the ignore file.
type Rule struct {
	Pattern  string
	Negate   bool
	BasePath string
	Source   string

	BasePathPrefix string
	Spec           CompiledSpec

	scope ruleScope
	// invalid marks a glob the matcher accepts but ripgrep rejects.
	invalid bool
}

// Options configures a Walker.
type Options struct {
	// Base patterns apply everywhere, below every ignore file. GitExcludes
	// are git's global excludes, which apply the same way but only inside a
	// repository. Both match relative to the repository root, or outside one
	// to the root of the filesystem.
	Base        []string
	GitExcludes []string
	// Overrides decide what they match ahead of every ignore file and
	// hidden-file skipping, whether or not IgnoreEnabled is set: the last
	// one to match a path wins, as with ripgrep's -g globs. WalkPath matches
	// them against the path from the working directory, as the root was
	// named; Walk against the path within the walked fs.FS, with
	// OverridePrefix, if any, before it.
	Overrides      []Override
	OverridePrefix string
	// MatchAsDir, when set, picks the files, by name, that ignore rules and
	// overrides match as directories, such as archives that are expanded
	// like directories.
	MatchAsDir func(name string) bool
	// IgnoreEnabled reads .ignore and .lxignore files and, inside a git
	// repository, .gitignore files and the repository's info/exclude.
	IgnoreEnabled bool
	// RepoRoot has Walk treat the root of the walked fs.FS as the root of
	// a git repository without its .git, such as a repository host's
	// archive of one: .gitignore files and GitExcludes apply below it.
	RepoRoot   bool
	SkipHidden bool
	// FollowDirSymlinks descends into symlinks that resolve to directories.
	// SkipFileSymlinks drops symlinks that resolve to files.
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	// Cache shares loaded ignore rules between the walkers of one run. Nil
	// gives the walker a cache of its own, on the host filesystem.
	Cache *Cache
	// OnIgnore, when set, is told about every path the walk skips and why.
	OnIgnore func(path, reason string)
	// Log is where the walk reports files it cannot read. Nil means the
	// Cache's logger, or slog.Default() without a Cache.
	Log *slog.Logger
}

// Walker traverses a directory tree, skipping what the ignore rules exclude.
type Walker struct {
	opts        Options
	base        []Rule
	gitExcludes []Rule
	baseKey     string
	overrides   *overrides
	cache       *Cache
	log         *slog.Logger
}

// New returns a Walker configured by opts.
func New(opts Options) *Walker {
	cache := opts.Cache
	if cache == nil {
		cache = NewCache(nil, opts.Log)
	}
	log := opts.Log
	if log == nil {
		log = cache.log
	}
	return &Walker{
		opts:        opts,
		base:        parseRules(opts.Base, "", "global/base"),
		gitExcludes: withScope(parseRules(opts.GitExcludes, "", "global git excludes"), scopeGit),
		baseKey:     strings.Join(opts.Base, "\n") + "\x00" + strings.Join(opts.GitExcludes, "\n"),
		overrides:   compileOverrides(opts.Overrides),
		cache:       cache,
		log:         log,
	}
}

// Accepts reports whether the path, named from the working directory or
// absolutely, is kept by the overrides as it would be in a walk from the
// working directory: neither it nor a directory above it is excluded, and in
// whitelist mode a file must be included. Ignore files are not consulted.
func (w *Walker) Accepts(path string, isDir bool) bool {
	if w.overrides == nil {
		return true
	}
	a := w.cache.Anchor(path)
	return w.overrides.accepts(a, a.Path("."), isDir)
}
