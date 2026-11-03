package walker

import (
	"fmt"
	"strings"
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

func parseRules(lines []string, basePath, source string) []Rule {
	var rules []Rule
	for _, p := range lines {
		p = trimTrailingSpaces(strings.TrimLeft(strings.TrimSuffix(p, "\r"), " \t"))
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}

		negate := false
		if strings.HasPrefix(p, "!") {
			negate = true
			p = strings.TrimPrefix(p, "!")
		} else if strings.HasPrefix(p, `\!`) || strings.HasPrefix(p, `\#`) {
			// An escaped leading ! or # is part of the pattern.
			p = p[1:]
		}
		// Backslashes escape, as in git, so the matcher keeps them.
		rules = append(rules, newRule(cleanPattern(p), negate, basePath, source))
	}
	return rules
}

// newRule compiles the pattern p, without a leading "!", matched relative to
// basePath.
func newRule(p string, negate bool, basePath, source string) Rule {
	basePathPrefix := ""
	if basePath != "" && basePath != "." {
		basePathPrefix = basePath + "/"
	}
	spec := compileNormalizedSpec(p)
	// A trailing "/**" matches inside a directory but not the directory
	// itself, as in git and ripgrep.
	if dir, ok := strings.CutSuffix(spec.Pattern.Pattern, "/**"); ok {
		spec.Pattern = compilePattern(dir + "/*/**")
	}
	return Rule{
		Pattern:        p,
		Negate:         negate,
		BasePath:       basePath,
		Source:         source,
		BasePathPrefix: basePathPrefix,
		Spec:           spec,
	}
}

// trimTrailingSpaces strips the trailing spaces of an ignore file line, as
// git does, except one escaped with a backslash, which is kept as a space.
func trimTrailingSpaces(p string) string {
	end := len(p)
	for end > 0 && p[end-1] == ' ' {
		if end >= 2 && p[end-2] == '\\' {
			return p[:end-2] + " "
		}
		end--
	}
	return p[:end]
}

// matchIgnoreSpec matches an ignore rule as git does: a pattern without a
// slash against the base name only. Parent directories were already judged
// by the walk, so they are not matched again.
func matchIgnoreSpec(spec CompiledSpec, ctx pathMatchInfo) bool {
	if spec.Pattern.HasSlash || spec.Anchored {
		return spec.Pattern.matchCandidate(ctx.relPath)
	}
	return spec.Pattern.matchCandidate(ctx.baseName)
}

// pathMatcher matches one path against rules, computing the relative path
// once per base path rather than per rule.
type pathMatcher struct {
	ctx pathMatchInfo

	base   string
	under  bool
	target pathMatchInfo
}

func newPathMatcher(relPath string) *pathMatcher {
	return &pathMatcher{ctx: buildPathMatchInfo(relPath), base: "\x00"}
}

func (m *pathMatcher) match(rule Rule, isDir bool) bool {
	if rule.Spec.DirOnly && !isDir {
		return false
	}
	if rule.BasePathPrefix == "" {
		return matchIgnoreSpec(rule.Spec, m.ctx)
	}
	if rule.BasePath != m.base {
		m.base = rule.BasePath
		switch {
		case m.ctx.relPath == rule.BasePath:
			m.under, m.target = true, dotPathMatchInfo
		case strings.HasPrefix(m.ctx.relPath, rule.BasePathPrefix):
			m.under, m.target = true, buildPathMatchInfo(m.ctx.relPath[len(rule.BasePathPrefix):])
		default:
			m.under = false
		}
	}
	return m.under && matchIgnoreSpec(rule.Spec, m.target)
}

// lastMatch returns what rules say about a path: the last of them to match
// it decides, ignoring it, or keeping it when it is a negation.
func lastMatch(relPath string, isDir bool, rules []Rule, wantReason bool) (match, string) {
	if len(rules) == 0 {
		return matchNone, ""
	}
	m := newPathMatcher(relPath)
	for i := len(rules) - 1; i >= 0; i-- {
		rule := &rules[i]
		if !m.match(*rule, isDir) {
			continue
		}
		if rule.Negate {
			return matchKeep, ""
		}
		if wantReason {
			return matchIgnore, fmt.Sprintf("rule %q in %s", rule.Pattern, rule.Source)
		}
		return matchIgnore, ""
	}
	return matchNone, ""
}
