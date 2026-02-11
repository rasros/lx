package lx

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Rule represents a single parsed line from the ignore file.
type Rule struct {
	Pattern  string
	Negate   bool
	BasePath string
	Source   string
}

// Walker configures the file traversal.
type Walker struct {
	BaseRules     []Rule
	OverrideRules []Rule
	IgnoreEnabled bool
	OnIgnore func(path string, reason string)
}

// NewWalker initializes the walker.
func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return &Walker{
		BaseRules:     parseRules(basePatterns, "", "global/base"),
		OverrideRules: parseRules(overridePatterns, "", "override flags"),
		IgnoreEnabled: true,
	}
}

// IsMatch checks if a path matches a pattern. Exposed for CLI filtering.
func IsMatch(pattern, relPath string) bool {
	relPath = path.Clean(strings.ReplaceAll(relPath, "\\", "/"))
	pattern = path.Clean(strings.ReplaceAll(pattern, "\\", "/"))

	name := path.Base(relPath)

	// Floating pattern (e.g. "*.go")
	if !strings.Contains(pattern, "/") {
		matched, _ := doublestar.Match(pattern, name)
		if matched {
			return true
		}
	}

	// Anchored pattern (e.g. "src/*.go")
	matchPattern := strings.TrimPrefix(pattern, "/")
	matched, _ := doublestar.Match(matchPattern, relPath)
	return matched
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

		p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
		p = strings.TrimRight(p, "/")

		rules = append(rules, Rule{
			Pattern:  p,
			Negate:   negate,
			BasePath: basePath,
			Source:   source,
		})
	}
	return rules
}

func match(rule Rule, name, relPath string) bool {
	targetPath := relPath
	pattern := rule.Pattern

	if rule.BasePath != "" && rule.BasePath != "." {
		if !strings.HasPrefix(relPath, rule.BasePath+"/") {
			return false
		}
		targetPath = strings.TrimPrefix(relPath, rule.BasePath+"/")
	}

	if !strings.Contains(pattern, "/") {
		matched, _ := doublestar.Match(pattern, name)
		if matched {
			return true
		}
	}

	matchPattern := strings.TrimPrefix(pattern, "/")
	matched, _ := doublestar.Match(matchPattern, targetPath)
	return matched
}

// checkIgnore determines if a path should be ignored and returns the reason.
func checkIgnore(relPath string, rules []Rule, parentIgnored bool) (bool, string) {
	ignored := parentIgnored
	reason := "parent directory"
	name := path.Base(relPath)

	for _, rule := range rules {
		if match(rule, name, relPath) {
			if rule.Negate {
				ignored = false
				reason = "" // explicitly included
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

		// Check Scope (BasePath)
		if rule.BasePath != "" && rule.BasePath != "." {
			if !strings.HasPrefix(dirPath, rule.BasePath) && !strings.HasPrefix(rule.BasePath, dirPath) {
				continue
			}
		}

		// Floating patterns (e.g. "*.go") match files in any subdirectory
		if !strings.Contains(rule.Pattern, "/") {
			return true
		}

		// Anchored Patterns
		fullPattern := rule.Pattern
		if rule.BasePath != "" && rule.BasePath != "." {
			fullPattern = rule.BasePath + "/" + rule.Pattern
		}
		fullPattern = strings.TrimPrefix(fullPattern, "/")

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

func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}

	if !info.IsDir() {
		var localRules []Rule
		if w.IgnoreEnabled {
			localRules = w.loadIgnoreFiles(fsys, ".")
		}
		effectiveRules := make([]Rule, 0, len(w.BaseRules)+len(localRules)+len(w.OverrideRules))
		effectiveRules = append(effectiveRules, w.BaseRules...)
		effectiveRules = append(effectiveRules, localRules...)
		effectiveRules = append(effectiveRules, w.OverrideRules...)

		isIgnored, reason := checkIgnore(root, effectiveRules, false)
		if isIgnored {
			if w.OnIgnore != nil {
				w.OnIgnore(root, reason)
			}
			return nil
		}
		return walkFn(root, dirEntryAdapter{info}, nil)
	}

	return w.recursiveWalk(fsys, root, w.BaseRules, walkFn, false)
}

func (w *Walker) recursiveWalk(fsys fs.FS, dir string, parentRules []Rule, walkFn fs.WalkDirFunc, parentIgnored bool) error {
	var localRules []Rule
	if w.IgnoreEnabled {
		localRules = w.loadIgnoreFiles(fsys, dir)
	}

	effectiveRules := make([]Rule, 0, len(parentRules)+len(localRules)+len(w.OverrideRules))
	effectiveRules = append(effectiveRules, parentRules...)
	effectiveRules = append(effectiveRules, localRules...)
	effectiveRules = append(effectiveRules, w.OverrideRules...)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}

	for _, d := range entries {
		childPath := d.Name()
		if dir != "." {
			childPath = path.Join(dir, d.Name())
		}

		isIgnored, reason := checkIgnore(childPath, effectiveRules, parentIgnored)

		if d.IsDir() {
			if isIgnored {
				if hasNestedException(childPath, effectiveRules) {
					if err := w.recursiveWalk(fsys, childPath, effectiveRules, walkFn, true); err != nil {
						return err
					}
					continue
				}
				if w.OnIgnore != nil {
					w.OnIgnore(childPath, reason)
				}
				continue
			}

			if err := walkFn(childPath, d, nil); err != nil {
				if err == fs.SkipDir {
					continue
				}
				return err
			}

			if err := w.recursiveWalk(fsys, childPath, effectiveRules, walkFn, false); err != nil {
				return err
			}

		} else {
			if isIgnored {
				if w.OnIgnore != nil {
					w.OnIgnore(childPath, reason)
				}
				continue
			}
			if err := walkFn(childPath, d, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Walker) loadIgnoreFiles(fsys fs.FS, dir string) []Rule {
	var rules []Rule
	files := []string{".gitignore", ".ignore", ".lxignore"}

	for _, name := range files {
		fp := name
		if dir != "." {
			fp = path.Join(dir, name)
		}

		data, err := fs.ReadFile(fsys, fp)
		if err == nil {
			sc := bufio.NewScanner(bytes.NewReader(data))
			var lines []string
			for sc.Scan() {
				lines = append(lines, sc.Text())
			}
			rules = append(rules, parseRules(lines, dir, name)...)
		}
	}
	return rules
}

type dirEntryAdapter struct {
	info fs.FileInfo
}

func (d dirEntryAdapter) Name() string               { return d.info.Name() }
func (d dirEntryAdapter) IsDir() bool                { return d.info.IsDir() }
func (d dirEntryAdapter) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d dirEntryAdapter) Info() (fs.FileInfo, error) { return d.info, nil }
