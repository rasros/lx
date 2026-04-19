package walker

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type pathMatchInfo struct {
	relPath  string
	baseName string
	parts    []string
}

var dotPathMatchInfo = pathMatchInfo{
	relPath:  ".",
	baseName: ".",
	parts:    []string{"."},
}

func buildPathMatchInfo(relPath string) pathMatchInfo {
	parts := strings.Split(relPath, "/")
	baseName := relPath
	if len(parts) > 0 {
		baseName = parts[len(parts)-1]
	}
	return pathMatchInfo{
		relPath:  relPath,
		baseName: baseName,
		parts:    parts,
	}
}

// IsMatch checks if a path matches a pattern. Exposed for CLI filtering.
func IsMatch(pattern, relPath string) bool {
	spec := compileSpec(pattern)
	ctx := buildPathMatchInfo(normalizePathForMatch(relPath))
	return matchCompiledSpec(spec, ctx)
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

		p = normalizePattern(p)
		spec := compileNormalizedSpec(p)

		rules = append(rules, Rule{
			Pattern:        p,
			Negate:         negate,
			BasePath:       basePath,
			Source:         source,
			BasePathPrefix: basePathPrefix,
			Spec:           spec,
		})
	}
	return rules
}

func matchCompiledSpec(spec CompiledSpec, ctx pathMatchInfo) bool {
	if spec.Pattern.HasSlash || spec.Anchored {
		return spec.Pattern.matchCandidate(ctx.relPath)
	}

	if spec.Pattern.matchCandidate(ctx.baseName) {
		return true
	}

	for i := 0; i < len(ctx.parts)-1; i++ {
		if spec.Pattern.matchCandidate(ctx.parts[i]) {
			return true
		}
	}

	return false
}

func match(rule Rule, relPath string, isDir bool) bool {
	return matchWithPathInfo(rule, buildPathMatchInfo(relPath), isDir)
}

func matchWithPathInfo(rule Rule, ctx pathMatchInfo, isDir bool) bool {
	if rule.Spec.DirOnly && !isDir {
		return false
	}

	target := ctx
	if rule.BasePathPrefix != "" {
		if !strings.HasPrefix(ctx.relPath, rule.BasePathPrefix) && ctx.relPath != rule.BasePath {
			return false
		}
		if ctx.relPath == rule.BasePath {
			target = dotPathMatchInfo
		} else {
			target = buildPathMatchInfo(strings.TrimPrefix(ctx.relPath, rule.BasePathPrefix))
		}
	}

	return matchCompiledSpec(rule.Spec, target)
}

// checkIgnore determines if a path should be ignored and returns the reason.
func checkIgnore(relPath string, isDir bool, rules []Rule, parentIgnored bool) (bool, string) {
	ignored := parentIgnored
	reason := "parent directory"
	ctx := buildPathMatchInfo(relPath)

	for _, rule := range rules {
		if matchWithPathInfo(rule, ctx, isDir) {
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

		pattern := rule.Spec.Pattern

		// Floating patterns (e.g. "*.go") match files in any subdirectory.
		if !pattern.HasSlash && !rule.Spec.Anchored {
			return true
		}

		// Anchored patterns.
		fullPattern := pattern.Pattern
		if rule.BasePathPrefix != "" {
			fullPattern = rule.BasePathPrefix + fullPattern
		}

		if pattern.HasDoubleStar {
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
