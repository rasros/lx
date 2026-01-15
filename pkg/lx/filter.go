package lx

import (
	"path"
	"strings"
)

func IsKept(p string, includes, excludes []string) bool {
	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if matchPattern(pattern, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range excludes {
		if matchPattern(pattern, p) {
			return false
		}
	}
	return true
}

func matchPattern(pattern, p string) bool {
	// Standardize path for matching
	p = path.Clean(p)

	if !strings.Contains(pattern, "/") {
		return match(pattern, path.Base(p))
	}
	return match(pattern, p)
}

func match(pattern, name string) bool {
	matched, _ := path.Match(pattern, name)
	return matched
}
