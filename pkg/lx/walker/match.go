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
	basePathPrefix := ""
	if basePath != "" && basePath != "." {
		basePathPrefix = basePath + "/"
	}

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

		// Backslashes escape the next character, as in git, so they are
		// kept for the matcher rather than read as separators.
		p = cleanPattern(p)
		spec := compileNormalizedSpec(p)
		// A trailing "/**" matches everything inside a directory but not the
		// directory itself, as in git and ripgrep, so "*" and "!src/**"
		// leave src/ ignored: at least one component must follow.
		if dir, ok := strings.CutSuffix(spec.Pattern.Pattern, "/**"); ok {
			spec.Pattern = compilePattern(dir + "/*/**")
		}

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

// matchIgnoreSpec matches an ignore rule as git does: a pattern without a
// slash against the base name only. A walk decides whether each directory
// above a path is ignored before entering it, so the directories' names are
// not matched again here, and the contents of a re-included directory, such
// as tags/ after "tags" and "!tags/", are not ignored by the first rule.
func matchIgnoreSpec(spec CompiledSpec, ctx pathMatchInfo) bool {
	if spec.Pattern.HasSlash || spec.Anchored {
		return spec.Pattern.matchCandidate(ctx.relPath)
	}
	return spec.Pattern.matchCandidate(ctx.baseName)
}

// pathMatcher matches one path against rules. Rules loaded from the same
// directory share a base path, so the path relative to it is worked out once
// per base path rather than once per rule.
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

// whitelisted reports whether the last of rules to match a path is a
// negation, which re-includes it even when it is hidden, as in ripgrep.
func whitelisted(relPath string, isDir bool, rules []Rule) (matched, negated bool) {
	m := newPathMatcher(relPath)
	for _, rule := range rules {
		if m.match(rule, isDir) {
			matched, negated = true, rule.Negate
		}
	}
	return matched, negated
}

// checkIgnore applies rules to a path, continuing from the state ignored
// and its reason left by earlier rules.
// The last matching rule decides.
func checkIgnore(relPath string, isDir bool, rules []Rule, ignored bool, reason string, wantReason bool) (bool, string) {
	if len(rules) == 0 {
		return ignored, reason
	}
	m := newPathMatcher(relPath)

	for _, rule := range rules {
		if m.match(rule, isDir) {
			if rule.Negate {
				ignored = false
				if wantReason {
					reason = ""
				}
			} else {
				ignored = true
				if wantReason {
					reason = fmt.Sprintf("rule %q in %s", rule.Pattern, rule.Source)
				}
			}
		}
	}
	return ignored, reason
}
