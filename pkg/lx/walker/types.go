package walker

import "strings"

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
	// Base patterns apply everywhere, below every ignore file. GitExcludes
	// are git's global excludes, which apply the same way but only inside a
	// repository. Both match relative to the repository root, or outside one
	// to the root of the filesystem.
	Base        []string
	GitExcludes []string
	// Override patterns apply after every ignore file, matched relative to
	// the walk root. They apply whether or not IgnoreEnabled is set.
	Override []string
	// IgnoreEnabled reads .ignore and .lxignore files and, inside a git
	// repository, .gitignore files and the repository's info/exclude.
	IgnoreEnabled bool
	SkipHidden    bool
	// FollowDirSymlinks descends into symlinks that resolve to directories.
	// SkipFileSymlinks drops symlinks that resolve to files.
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	// Cache shares loaded ignore rules between the walkers of one run. Nil
	// gives the walker a cache of its own, on the host filesystem.
	Cache *Cache
	// OnIgnore, when set, is told about every path the walk skips and why.
	OnIgnore func(path, reason string)
}

// Walker traverses a directory tree, skipping what the ignore rules exclude.
type Walker struct {
	opts        Options
	base        []Rule
	gitExcludes []Rule
	baseKey     string
	override    []Rule
	cache       *Cache
}

// New returns a Walker configured by opts.
func New(opts Options) *Walker {
	cache := opts.Cache
	if cache == nil {
		cache = NewCache(nil)
	}
	return &Walker{
		opts:        opts,
		base:        parseRules(opts.Base, "", "global/base"),
		gitExcludes: withScope(parseRules(opts.GitExcludes, "", "global git excludes"), scopeGit),
		baseKey:     strings.Join(opts.Base, "\n") + "\x00" + strings.Join(opts.GitExcludes, "\n"),
		override:    parseRules(opts.Override, "", "override flags"),
		cache:       cache,
	}
}
