package walker

// IsKept checks if a path matches include patterns and does not match exclude patterns.
func IsKept(p string, includes, excludes []string) bool {
	ctx := buildPathMatchInfo(normalizePathForMatch(p))

	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if matchCompiledSpec(compileSpec(pattern), ctx) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range excludes {
		if matchCompiledSpec(compileSpec(pattern), ctx) {
			return false
		}
	}

	return true
}
