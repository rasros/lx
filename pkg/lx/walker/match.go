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

	isAnchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	if strings.Contains(pattern, "/") || isAnchored {
		matched, _ := doublestar.Match(pattern, relPath)
		return matched
	}

	name := path.Base(relPath)
	if matched, _ := doublestar.Match(pattern, name); matched {
		return true
	}

	parts := strings.Split(relPath, "/")
	for i, part := range parts {
		if isDirOnly && i == len(parts)-1 {
			continue
		}
		if matched, _ := doublestar.Match(pattern, part); matched {
			return true
		}
	}
	return false
}

func parseRules(lines []string, basePath, source string) []Rule {
	var rules []Rule
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

		rules = append(rules, Rule{
			Pattern:   p,
			Negate:    negate,
			IsLiteral: !strings.ContainsAny(p, "*?[{"),
			BasePath:  basePath,
			Source:    source,
		})
	}
	return rules
}

func match(rule Rule, relPath string, isDir bool) bool {
	targetPath := relPath
	pattern := rule.Pattern

	isDirOnly := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")

	if isDirOnly && !isDir {
		return false
	}

	if rule.BasePath != "" && rule.BasePath != "." {
		if !strings.HasPrefix(relPath, rule.BasePath+"/") && relPath != rule.BasePath {
			return false
		}
		if relPath == rule.BasePath {
			targetPath = "."
		} else {
			targetPath = strings.TrimPrefix(relPath, rule.BasePath+"/")
		}
	}

	isAnchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	if strings.Contains(pattern, "/") || isAnchored {
		if rule.IsLiteral {
			return pattern == targetPath
		}
		matched, _ := doublestar.Match(pattern, targetPath)
		return matched
	}

	name := path.Base(targetPath)
	if rule.IsLiteral {
		if pattern == name {
			return true
		}
	} else if matched, _ := doublestar.Match(pattern, name); matched {
		return true
	}

	start := 0
	for i := 0; i <= len(targetPath); i++ {
		if i == len(targetPath) || targetPath[i] == '/' {
			part := targetPath[start:i]
			start = i + 1
			if rule.IsLiteral {
				if pattern == part {
					return true
				}
			} else if matched, _ := doublestar.Match(pattern, part); matched {
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

		pattern := strings.TrimSuffix(rule.Pattern, "/")
		isAnchored := strings.HasPrefix(pattern, "/")
		pattern = strings.TrimPrefix(pattern, "/")

		// Floating patterns (e.g. "*.go") match files in any subdirectory.
		if !strings.Contains(pattern, "/") && !isAnchored {
			return true
		}

		// Anchored patterns.
		fullPattern := pattern
		if rule.BasePath != "" && rule.BasePath != "." {
			fullPattern = rule.BasePath + "/" + pattern
		}

		if strings.Contains(fullPattern, "**") {
			return true
		}

		ruleParts := strings.Split(fullPattern, "/")
		if len(dirParts) >= len(ruleParts) {
			continue
		}

		isParent := true
		for i, part := range dirParts {
			rulePart := ruleParts[i]
			matched, _ := doublestar.Match(rulePart, part)
			if !matched {
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
