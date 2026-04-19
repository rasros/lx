package walker

// Rule represents a single parsed line from the ignore file.
type Rule struct {
	Pattern   string
	Negate    bool
	IsLiteral bool
	BasePath  string
	Source    string

	// Precomputed fields to keep the match hot path allocation-light.
	MatchPattern   string
	PatternValid   bool
	DirOnly        bool
	Anchored       bool
	HasSlash       bool
	HasDoubleStar  bool
	BasePathPrefix string
	// Fast prefilter for patterns that only use '*' wildcards.
	OnlyStarWildcards bool
	LiteralPrefix     string
	LiteralSuffix     string
}

// Walker configures the file traversal.
type Walker struct {
	BaseRules     []Rule
	OverrideRules []Rule
	IgnoreEnabled bool
	OnIgnore      func(path string, reason string)
}

// NewWalker initializes the walker.
func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return &Walker{
		BaseRules:     parseRules(basePatterns, "", "global/base"),
		OverrideRules: parseRules(overridePatterns, "", "override flags"),
		IgnoreEnabled: true,
	}
}
