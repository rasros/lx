package lx

import (
	"path/filepath"
	"strings"
)

// IsKept returns true if the path should be included based on include/exclude patterns.
func IsKept(path string, includes, excludes []string) bool {
	// 1. If includes are specified, the path MUST match at least one
	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if matchPattern(pattern, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 2. If excludes are specified, the path MUST NOT match any
	for _, pattern := range excludes {
		if matchPattern(pattern, path) {
			return false
		}
	}

	return true
}

func matchPattern(pattern, path string) bool {
	// If the pattern has no separators, match against the base filename
	// e.g. -e "*.go" matches "cmd/main.go"
	if !strings.Contains(pattern, string(filepath.Separator)) {
		name := filepath.Base(path)
		matched, _ := filepath.Match(pattern, name)
		return matched
	}

	// Otherwise, match against the relative path provided by walker
	// e.g. -e "cmd/*" matches "cmd/main.go"
	matched, _ := filepath.Match(pattern, path)
	return matched
}
