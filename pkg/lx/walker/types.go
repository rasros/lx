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
}

// Options configures a Walker.
type Options struct {
	// Base patterns apply everywhere, below every ignore file; GitExcludes
	// only inside a repository. Both match relative to the repository root,
	// or else the filesystem root.
	Base        []string
	GitExcludes []string
	// Overrides decide what they match ahead of ignore files and hidden-file
	// skipping; the last match wins, as with ripgrep's -g globs.
	Overrides      []Override
	OverridePrefix string
	// MatchAsDir picks the files matched as directories, such as archives
	// that are expanded.
	MatchAsDir func(name string) bool
	// IgnoreEnabled reads .ignore and .lxignore files and, inside a git
	// repository, .gitignore files and the repository's info/exclude.
	IgnoreEnabled bool
	// RepoRoot has Walk treat the fs.FS root as a git repository root
	// without its .git, as in a repository host's archive.
	RepoRoot   bool
	SkipHidden bool
	// FollowDirSymlinks descends into symlinks that resolve to directories.
	// SkipFileSymlinks drops symlinks that resolve to files.
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	// Cache shares loaded ignore rules between walkers. Nil gives the walker
	// its own.
	Cache *Cache
	// OnIgnore, when set, is told about every path the walk skips and why.
	OnIgnore func(path, reason string)
	// Log reports unreadable files. Nil means the Cache's logger or
	// slog.Default().
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

// ShowHidden returns a copy of w that does not skip hidden paths.
func (w *Walker) ShowHidden() *Walker {
	c := *w
	c.opts.SkipHidden = false
	return &c
}

// Accepts reports whether the overrides keep path as a walk from the
// working directory would. Ignore files are not consulted.
func (w *Walker) Accepts(path string, isDir bool) bool {
	if w.overrides == nil {
		return true
	}
	a := w.cache.Anchor(path)
	return w.overrides.accepts(a.Path("."), isDir)
}
