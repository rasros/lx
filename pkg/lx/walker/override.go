package walker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// OverrideKind says what an override glob does to the paths it matches.
type OverrideKind uint8

const (
	// Include keeps what its glob matches, as ripgrep's -g glob does, and
	// puts the walk in whitelist mode: a file no Include or Keep glob matches
	// is dropped.
	Include OverrideKind = iota
	// Exclude drops what its glob matches, as ripgrep's -g '!glob' does.
	Exclude
	// Keep keeps what its glob matches, as Include does, without putting the
	// walk in whitelist mode.
	Keep
)

// Override is one of lx's -i and -e globs, which ripgrep calls overrides:
// the last one to match a path decides it, ahead of ignore files and
// hidden-file skipping. Glob has gitignore syntax; Kind takes the place of
// the one leading "!" IncludeOverride and ExcludeOverride strip, so Glob may
// still start with "!": IncludeOverride("!!x") is an Exclude of "!x".
type Override struct {
	Glob string
	Kind OverrideKind
}

// IncludeOverride returns the override of lx's -i pattern p, which is
// ripgrep's -g p: an Include, or an Exclude of what follows a leading "!".
func IncludeOverride(p string) Override {
	if rest, ok := strings.CutPrefix(p, "!"); ok {
		return Override{Glob: rest, Kind: Exclude}
	}
	return Override{Glob: p, Kind: Include}
}

// ExcludeOverride returns the override of lx's -e pattern p, which is
// ripgrep's -g '!p': an Exclude, or a Keep of what follows a leading "!".
func ExcludeOverride(p string) Override {
	if rest, ok := strings.CutPrefix(p, "!"); ok {
		return Override{Glob: rest, Kind: Keep}
	}
	return Override{Glob: p, Kind: Exclude}
}

// match is what a set of rules says about a path.
type match uint8

const (
	matchNone match = iota
	matchIgnore
	matchKeep
)

// overrides are compiled override globs. Each is a rule, negated when it
// keeps what it matches, matched against the path from the working
// directory.
type overrides struct {
	rules []Rule
	// whitelist is set by an Include: a file no rule keeps is dropped.
	whitelist bool
	// keep are the globs that can keep a file, which decide whether a walk
	// in whitelist mode needs to enter a directory.
	keep []CompiledSpec
}

func compileOverrides(globs []Override) *overrides {
	if len(globs) == 0 {
		return nil
	}
	o := &overrides{}
	for _, g := range globs {
		r := g.rule()
		o.rules = append(o.rules, r)
		if g.Kind == Include {
			o.whitelist = true
		}
		if g.Kind != Exclude && !r.Spec.DirOnly {
			o.keep = append(o.keep, r.Spec)
		}
	}
	return o
}

// rule compiles o's glob as a gitignore pattern, as written.
func (o Override) rule() Rule {
	p := o.Glob
	if filepath.Separator == '\\' {
		// As in ripgrep, a backslash is a separator on Windows.
		p = strings.ReplaceAll(p, `\`, "/")
	}
	return newRule(p, o.Kind != Exclude, "", "globs")
}

// Validate reports an error when o's glob cannot be parsed, such as an
// unclosed "[" or "{", an unopened "}" or a dangling "\".
func (o Override) Validate() error {
	if !doublestar.ValidatePattern(o.rule().Spec.Pattern.Pattern) {
		return fmt.Errorf("invalid glob %q", o.Glob)
	}
	return nil
}

// match returns what the overrides say about the path p, from the working
// directory: the last rule to match it decides, and in whitelist mode a
// file none matches is ignored.
func (o *overrides) match(p string, isDir, wantReason bool) (match, string) {
	if m, reason := lastMatch(p, isDir, o.rules, wantReason); m != matchNone {
		return m, reason
	}
	if o.whitelist && !isDir {
		return matchIgnore, "matched by no include glob"
	}
	return matchNone, ""
}

// reachBelow reports whether a walk must enter the directory at dir, from
// the working directory, to find every file the overrides keep: not in
// whitelist mode when no glob that keeps files could match below it.
func (o *overrides) reachBelow(dir string) bool {
	if !o.whitelist || dir == "." {
		return true
	}
	for _, spec := range o.keep {
		if couldMatchDescendant(spec, dir) {
			return true
		}
	}
	return false
}

// accepts reports whether the path p would be found by a walk: neither p nor
// a directory above it that the walk judges is ignored. A walk judges the
// directories below its root, so the directories of an absolute path, and
// the leading ".." of a relative one, are not judged, as those of a root
// named that way are not.
func (o *overrides) accepts(p string, isDir bool) bool {
	if p == "." {
		return true
	}
	if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
		start := 0
		for strings.HasPrefix(p[start:], "../") {
			start += 3
		}
		for i := start + 1; i < len(p); i++ {
			if p[i] != '/' {
				continue
			}
			if m, _ := o.match(p[:i], true, false); m == matchIgnore {
				return false
			}
		}
	}
	m, _ := o.match(p, isDir, false)
	return m != matchIgnore && (!isDir || o.reachBelow(p))
}
