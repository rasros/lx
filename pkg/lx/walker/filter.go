package walker

import (
	"path/filepath"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

type filterPattern struct {
	pattern      string
	hasMagic     bool
	hasSlash     bool
	patternValid bool
}

var filterPatternCache sync.Map // map[string]filterPattern

func compiledFilterPattern(pattern string) filterPattern {
	if v, ok := filterPatternCache.Load(pattern); ok {
		return v.(filterPattern)
	}

	fp := filterPattern{pattern: pattern}
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[', '{', '\\':
			fp.hasMagic = true
		case '/':
			fp.hasSlash = true
		}
	}
	fp.patternValid = !fp.hasMagic || doublestar.ValidatePattern(pattern)

	actual, _ := filterPatternCache.LoadOrStore(pattern, fp)
	return actual.(filterPattern)
}

// IsKept checks if a path matches include patterns and does not match exclude patterns.
func IsKept(p string, includes, excludes []string) bool {
	osPath := filepath.FromSlash(p)
	base := filepath.Base(osPath)

	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if fastMatch(pattern, osPath, base) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range excludes {
		if fastMatch(pattern, osPath, base) {
			return false
		}
	}

	return true
}

// fastMatch optimizes pattern matching for common cases to avoid expensive glob parsing.
func fastMatch(pattern, fullPath, baseName string) bool {
	if pattern == fullPath {
		return true
	}

	fp := compiledFilterPattern(pattern)

	if !fp.hasMagic {
		if fp.hasSlash {
			return false
		}
		return fp.pattern == baseName
	}

	if !fp.patternValid {
		return false
	}

	if fp.hasSlash {
		if doublestar.MatchUnvalidated(fp.pattern, fullPath) {
			return true
		}
	} else {
		if doublestar.MatchUnvalidated(fp.pattern, baseName) {
			return true
		}
		if doublestar.MatchUnvalidated(fp.pattern, fullPath) {
			return true
		}
	}

	return false
}
