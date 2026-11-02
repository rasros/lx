package walker

import (
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

// CompiledPattern stores precomputed data for a glob/literal pattern.
type CompiledPattern struct {
	Pattern           string
	IsLiteral         bool
	PatternValid      bool
	HasSlash          bool
	HasDoubleStar     bool
	OnlyStarWildcards bool
	LiteralPrefix     string
	LiteralSuffix     string
}

// CompiledSpec captures matching behavior around a compiled pattern.
type CompiledSpec struct {
	DirOnly  bool
	Anchored bool
	Pattern  CompiledPattern
}

var (
	compiledPatternCache sync.Map // map[string]CompiledPattern
	compiledSpecCache    sync.Map // map[string]CompiledSpec (key: normalized pattern)
)

func compilePattern(pattern string) CompiledPattern {
	if v, ok := compiledPatternCache.Load(pattern); ok {
		return v.(CompiledPattern)
	}

	cp := CompiledPattern{
		Pattern:       pattern,
		IsLiteral:     !strings.ContainsAny(pattern, "*?[{\\"),
		HasSlash:      strings.Contains(pattern, "/"),
		HasDoubleStar: strings.Contains(pattern, "**"),
	}
	cp.PatternValid = cp.IsLiteral || doublestar.ValidatePattern(pattern)
	cp.OnlyStarWildcards = !cp.IsLiteral && hasOnlyStarWildcards(pattern) && !cp.HasDoubleStar
	if cp.OnlyStarWildcards {
		cp.LiteralPrefix, cp.LiteralSuffix = starWildcardPrefixSuffix(pattern)
	}

	actual, _ := compiledPatternCache.LoadOrStore(pattern, cp)
	return actual.(CompiledPattern)
}

func compileSpec(rawPattern string) CompiledSpec {
	normalized := normalizePattern(rawPattern)
	if v, ok := compiledSpecCache.Load(normalized); ok {
		return v.(CompiledSpec)
	}

	spec := compileNormalizedSpec(normalized)
	actual, _ := compiledSpecCache.LoadOrStore(normalized, spec)
	return actual.(CompiledSpec)
}

// CompileSpecs compiles all filter patterns.
func CompileSpecs(rawPatterns []string) []CompiledSpec {
	if len(rawPatterns) == 0 {
		return nil
	}
	specs := make([]CompiledSpec, 0, len(rawPatterns))
	for _, p := range rawPatterns {
		specs = append(specs, compileSpec(p))
	}
	return specs
}

func compileNormalizedSpec(normalizedPattern string) CompiledSpec {
	dirOnly := strings.HasSuffix(normalizedPattern, "/")
	trimmed := strings.TrimSuffix(normalizedPattern, "/")
	anchored := strings.HasPrefix(trimmed, "/")
	matchPattern := strings.TrimPrefix(trimmed, "/")

	return CompiledSpec{
		DirOnly:  dirOnly,
		Anchored: anchored,
		Pattern:  compilePattern(matchPattern),
	}
}

// normalizePattern cleans an -i or -e pattern. On Windows a backslash there
// is a path separator; elsewhere, as in an ignore file, it escapes the next
// character.
func normalizePattern(raw string) string {
	if filepath.Separator == '\\' {
		raw = strings.ReplaceAll(raw, "\\", "/")
	}
	return cleanPattern(raw)
}

// cleanPattern cleans a pattern, keeping a trailing slash, which makes it
// match directories only.
func cleanPattern(raw string) string {
	hasTrailingSlash := strings.HasSuffix(raw, "/")
	cleaned := path.Clean(raw)
	if hasTrailingSlash && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func normalizePathForMatch(raw string) string {
	if filepath.Separator == '\\' {
		raw = strings.ReplaceAll(raw, "\\", "/")
	}
	return path.Clean(raw)
}

// IsMatchAnyCompiled checks whether relPath matches at least one compiled pattern.
func IsMatchAnyCompiled(specs []CompiledSpec, relPath string) bool {
	if len(specs) == 0 {
		return true
	}
	ctx := buildPathMatchInfo(normalizePathForMatch(relPath))
	for _, spec := range specs {
		if matchCompiledSpec(spec, ctx) {
			return true
		}
	}
	return false
}

// CouldMatchAnyDescendant reports whether at least one compiled pattern might match
// a file at or under dirRelPath. Returning false is a safe prune signal.
func CouldMatchAnyDescendant(specs []CompiledSpec, dirRelPath string) bool {
	if len(specs) == 0 {
		return true
	}
	dir := normalizePathForMatch(dirRelPath)
	if dir == "." {
		return true
	}
	for _, spec := range specs {
		if couldMatchDescendant(spec, dir) {
			return true
		}
	}
	return false
}

// couldMatchDescendant reports whether spec could match a file below dir. A
// pattern with a slash matches whole paths, so its leading segments must
// match dir's directories one by one, and a file below dir has more segments
// than dir. A "**" segment, or a brace group holding "**" or a slash, can
// span any number of them, so a pattern with one is not pruned past it.
func couldMatchDescendant(spec CompiledSpec, dir string) bool {
	// Floating patterns (e.g. "*.go") can match files in any subtree.
	if !spec.Pattern.HasSlash && !spec.Anchored {
		return true
	}
	pattern := spec.Pattern.Pattern
	if bracesSpanDirs(pattern) {
		return true
	}
	segs := strings.Split(pattern, "/")
	for i, name := range strings.Split(dir, "/") {
		if segs[i] == "**" {
			return true
		}
		if i == len(segs)-1 {
			// What is left of the pattern names the file itself.
			return false
		}
		ok, err := doublestar.Match(segs[i], name)
		if err != nil {
			return true
		}
		if !ok {
			return false
		}
	}
	return true
}

// bracesSpanDirs reports whether a brace group in pattern holds a slash or
// "**", either of which lets it match across directories, which splitting
// the pattern into segments would miss.
func bracesSpanDirs(pattern string) bool {
	depth := 0
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '/':
			if depth > 0 {
				return true
			}
		case '*':
			if depth > 0 && i+1 < len(pattern) && pattern[i+1] == '*' {
				return true
			}
		}
	}
	return false
}

func hasOnlyStarWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '?', '[', '{', '\\':
			return false
		}
	}
	return true
}

func starWildcardPrefixSuffix(pattern string) (string, string) {
	firstStar := strings.IndexByte(pattern, '*')
	if firstStar < 0 {
		return pattern, pattern
	}
	lastStar := strings.LastIndexByte(pattern, '*')
	return pattern[:firstStar], pattern[lastStar+1:]
}

func (cp CompiledPattern) matchCandidate(candidate string) bool {
	if cp.IsLiteral {
		return cp.Pattern == candidate
	}
	if !cp.PatternValid {
		return false
	}
	if cp.OnlyStarWildcards {
		if cp.LiteralPrefix != "" && !strings.HasPrefix(candidate, cp.LiteralPrefix) {
			return false
		}
		if cp.LiteralSuffix != "" && !strings.HasSuffix(candidate, cp.LiteralSuffix) {
			return false
		}
	}
	return doublestar.MatchUnvalidated(cp.Pattern, candidate)
}
