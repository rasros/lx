package walker

// Rule represents a single parsed line from the ignore file.
type Rule struct {
	Pattern  string
	Negate   bool
	BasePath string
	Source   string

	BasePathPrefix string
	Spec           CompiledSpec

	// PathOffset is set on rules loaded from a directory above the walk
	// root: the walk root's path relative to that directory, with a trailing
	// slash. Walk paths are prefixed with it before matching.
	PathOffset string
}

// Walker configures the file traversal.
type Walker struct {
	BaseRules []Rule
	// AncestorRules come from ignore files above the walk root, see
	// LoadAncestorRules. They apply after BaseRules and before the walk's
	// own ignore files.
	AncestorRules []Rule
	OverrideRules []Rule
	IgnoreEnabled bool
	SkipHidden    bool

	// FollowDirSymlinks descends into symlinks that resolve to directories.
	// SkipFileSymlinks drops symlinks that resolve to files.
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	OnIgnore          func(path string, reason string)
}

// NewWalker initializes the walker.
func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return &Walker{
		BaseRules:     parseRules(basePatterns, "", "global/base"),
		OverrideRules: parseRules(overridePatterns, "", "override flags"),
		IgnoreEnabled: true,
	}
}
