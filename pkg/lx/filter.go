package lx

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsKept checks if a path matches include patterns and does not match exclude patterns.
func IsKept(p string, includes, excludes []string) bool {
	osPath := filepath.FromSlash(p)

	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if matchPattern(filepath.FromSlash(pattern), osPath) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range excludes {
		if matchPattern(filepath.FromSlash(pattern), osPath) {
			return false
		}
	}
	return true
}

func matchPattern(pattern, p string) bool {
	p = filepath.Clean(p)

	if !strings.Contains(pattern, string(filepath.Separator)) {
		return match(pattern, filepath.Base(p))
	}
	return match(pattern, p)
}

func match(pattern, name string) bool {
	matched, _ := doublestar.Match(pattern, name)
	return matched
}
