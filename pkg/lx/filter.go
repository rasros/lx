package lx

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

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
		if hasSlash {
			return false
		}
		return pattern == baseName
	}

	if hasSlash {
		if m, _ := doublestar.Match(pattern, fullPath); m {
			return true
		}
	} else {
		if m, _ := doublestar.Match(pattern, baseName); m {
			return true
		}
		if m, _ := doublestar.Match(pattern, fullPath); m {
			return true
		}
	}

	return false
}
