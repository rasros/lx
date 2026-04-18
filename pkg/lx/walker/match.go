package walker

import (
	"fmt"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsMatch checks if a path matches a pattern. Exposed for CLI filtering.
func IsMatch(pattern, relPath string) bool {
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	isDirOnly := strings.HasSuffix(pattern, "/")

	relPath = path.Clean(relPath)
	pattern = path.Clean(pattern)

	pattern = strings.TrimSuffix(pattern, "/")
	isAnchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	isLiteral := !strings.ContainsAny(pattern, "*?[{")
	patternValid := isLiteral || doublestar.ValidatePattern(pattern)

	if strings.Contains(pattern, "/") || isAnchored {
		if isLiteral {
			return pattern == relPath
		}
		if !patternValid {
			return false
		}
		return doublestar.MatchUnvalidated(pattern, relPath)
	}

	name := path.Base(relPath)
	if isLiteral {
		if pattern == name {
			return true
		}
	} else {
		if !patternValid {
			return false
		}
		if doublestar.MatchUnvalidated(pattern, name) {
			return true
		}
	}

	start := 0
	for i := 0; i <= len(relPath); i++ {
		if i == len(relPath) || relPath[i] == '/' {
			part := relPath[start:i]
			start = i + 1

			if isDirOnly && i == len(relPath) {
				continue
			}
			if isLiteral {
				if pattern == part {
					return true
				}
			} else if doublestar.MatchUnvalidated(pattern, part) {
				return true
			}
		}
	}
	return false
}

func parseRules(lines []string, basePath, source string) []Rule {
	var rules []Rule
	basePathPrefix := ""
	if basePath != "" && basePath != "." {
		basePathPrefix = basePath + "/"
	}

	for _, p := range lines {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}

		negate := false
		if strings.HasPrefix(p, "!") {
			negate = true
			p = strings.TrimPrefix(p, "!")
		}

		p = strings.ReplaceAll(p, "\\", "/")
		hasTrailingSlash := strings.HasSuffix(p, "/")
		p = path.Clean(p)
		if hasTrailingSlash && p != "/" {
			p += "/"
		}

		isLiteral := !strings.ContainsAny(p, "*?[{")
		matchPattern := strings.TrimSuffix(p, "/")
		isAnchored := strings.HasPrefix(matchPattern, "/")
		matchPattern = strings.TrimPrefix(matchPattern, "/")

		patternValid := true
		if !isLiteral {
			patternValid = doublestar.ValidatePattern(matchPattern)
		}

		rules = append(rules, Rule{
			Pattern:        p,
			Negate:         negate,
			IsLiteral:      isLiteral,
			BasePath:       basePath,
			Source:         source,
			MatchPattern:   matchPattern,
			PatternValid:   patternValid,
			DirOnly:        strings.HasSuffix(p, "/"),
			Anchored:       isAnchored,
			HasSlash:       strings.Contains(matchPattern, "/"),
			HasDoubleStar:  strings.Contains(matchPattern, "**"),
			BasePathPrefix: basePathPrefix,
		})
	}
	return rules
}

func match(rule Rule, relPath string, isDir bool) bool {
	targetPath := relPath

	if rule.DirOnly && !isDir {
		return false
	}

	if rule.BasePathPrefix != "" {
		if !strings.HasPrefix(relPath, rule.BasePathPrefix) && relPath != rule.BasePath {
			return false
		}
		if relPath == rule.BasePath {
			targetPath = "."
		} else {
			targetPath = strings.TrimPrefix(relPath, rule.BasePathPrefix)
		}
	}

	if rule.HasSlash || rule.Anchored {
		if rule.IsLiteral {
			return rule.MatchPattern == targetPath
		}
		if !rule.PatternValid {
			return false
		}
		return doublestar.MatchUnvalidated(rule.MatchPattern, targetPath)
	}

	name := path.Base(targetPath)
	if rule.IsLiteral {
		if rule.MatchPattern == name {
			return true
		}
	} else {
		if !rule.PatternValid {
			return false
		}
		if doublestar.MatchUnvalidated(rule.MatchPattern, name) {
			return true
		}
	}

	start := 0
	for i := 0; i <= len(targetPath); i++ {
		if i == len(targetPath) || targetPath[i] == '/' {
			part := targetPath[start:i]
			start = i + 1
			if rule.IsLiteral {
				if rule.MatchPattern == part {
					return true
				}
			} else if doublestar.MatchUnvalidated(rule.MatchPattern, part) {
				return true
			}
		}
	}

	return false
}

// checkIgnore determines if a path should be ignored and returns the reason.
func checkIgnore(relPath string, isDir bool, rules []Rule, parentIgnored bool) (bool, string) {
	ignored := parentIgnored
	reason := "parent directory"

	for _, rule := range rules {
		if match(rule, relPath, isDir) {
			if rule.Negate {
				ignored = false
				reason = ""
			} else {
				ignored = true
				reason = fmt.Sprintf("rule %q in %s", rule.Pattern, rule.Source)
			}
		}
	}
	return ignored, reason
}

func hasNestedException(dirPath string, rules []Rule) bool {
	dirParts := strings.Split(dirPath, "/")

	for _, rule := range rules {
		if !rule.Negate {
			continue
		}

		if rule.BasePath != "" && rule.BasePath != "." {
			if !strings.HasPrefix(dirPath, rule.BasePath) && !strings.HasPrefix(rule.BasePath, dirPath) {
				continue
			}
		}

		pattern := rule.MatchPattern

		// Floating patterns (e.g. "*.go") match files in any subdirectory.
		if !rule.HasSlash && !rule.Anchored {
			return true
		}

		// Anchored patterns.
		fullPattern := pattern
		if rule.BasePathPrefix != "" {
			fullPattern = rule.BasePathPrefix + pattern
		}

		if rule.HasDoubleStar {
			return true
		}

		ruleParts := strings.Split(fullPattern, "/")
		if len(dirParts) >= len(ruleParts) {
			continue
		}

		isParent := true
		for i, part := range dirParts {
			rulePart := ruleParts[i]
			if rulePart == part {
				continue
			}
			if !doublestar.ValidatePattern(rulePart) || !doublestar.MatchUnvalidated(rulePart, part) {
				isParent = false
				break
			}
		}

		if isParent {
			return true
		}
	}
	return false
}
