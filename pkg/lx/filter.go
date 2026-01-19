package lx

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

// IsKept checks if a path matches include patterns and does not match exclude patterns.
func IsKept(p string, includes, excludes []string) bool {
	// Optimization 3: Standardize path once
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
	// 1. Exact match (fastest)
	if pattern == fullPath {
		return true
	}

	// 2. Check for Glob characters
	// If the pattern has no magic characters, it's a literal match we already failed above,
	// UNLESS it matched the basename (e.g. pattern "file.txt" matches "src/file.txt")
	hasMagic := false
	hasSlash := false
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[', '{', '\\':
			hasMagic = true
		case '/':
			hasSlash = true
		}
	}

	if !hasMagic {
		// Literal match logic
		if hasSlash {
			// Pattern has directory separators, must match full path (checked above)
			return false
		}
		// Pattern has no separators, match against basename
		return pattern == baseName
	}

	// 3. Fallback to full Doublestar match
	// We use Match instead of PathMatch because we've normalized slashes
	if hasSlash {
		if m, _ := doublestar.Match(pattern, fullPath); m {
			return true
		}
	} else {
		// If pattern has no slash, it can match basename
		if m, _ := doublestar.Match(pattern, baseName); m {
			return true
		}
		// Or it can match the full path (e.g. "**/foo")
		if m, _ := doublestar.Match(pattern, fullPath); m {
			return true
		}
	}

	return false
}
