package lx

import (
	"bufio"
	"bytes"
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
}

// Walker configures the file traversal.
type Walker struct {
	BaseRules     []Rule
	OverrideRules []Rule
	IgnoreEnabled bool
}

// NewWalker initializes the walker.
func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return &Walker{
		BaseRules:     parseRules(basePatterns, ""),
		OverrideRules: parseRules(overridePatterns, ""),
		IgnoreEnabled: true,
	}
}

// IsMatch checks if a path matches a pattern. Exposed for CLI filtering.
func IsMatch(pattern, relPath string) bool {
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	isDirOnly := strings.HasSuffix(pattern, "/")

	relPath = path.Clean(relPath)
	pattern = path.Clean(pattern)

	isAnchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	// 1. Anchored or contains slash (matches from root)
	if strings.Contains(pattern, "/") || isAnchored {
		matched, _ := doublestar.Match(pattern, relPath)
		return matched
	}

	// 2. Basename match
	name := path.Base(relPath)
	if matched, _ := doublestar.Match(pattern, name); matched {
		return true
	}

	// 3. Ubiquitous match (matches any directory segment in the path)
	parts := strings.Split(relPath, "/")
	for i, part := range parts {
		if isDirOnly && i == len(parts)-1 {
			continue // Skip matching the final segment if pattern explicitly targets directories
		}
		if matched, _ := doublestar.Match(pattern, part); matched {
			return true
		}
	}
	return false
}

func parseRules(lines []string, basePath string) []Rule {
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
			Pattern:  p,
			Negate:   negate,
			BasePath: basePath,
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

	// 1. Anchored or contains slash
	if strings.Contains(pattern, "/") || isAnchored {
		matched, _ := doublestar.Match(pattern, targetPath)
		return matched
	}

	// 2. Basename match
	name := path.Base(targetPath)
	if matched, _ := doublestar.Match(pattern, name); matched {
		return true
	}

	// 3. Ubiquitous match
	parts := strings.Split(targetPath, "/")
	for _, part := range parts {
		if matched, _ := doublestar.Match(pattern, part); matched {
			return true
		}
	}

	return false
}

func shouldIgnore(relPath string, isDir bool, rules []Rule, parentIgnored bool) bool {
	ignored := parentIgnored
	for _, rule := range rules {
		if match(rule, relPath, isDir) {
			ignored = !rule.Negate
		}
	}
	return ignored
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

		pattern := strings.TrimSuffix(rule.Pattern, "/")
		isAnchored := strings.HasPrefix(pattern, "/")
		pattern = strings.TrimPrefix(pattern, "/")

		// Floating patterns (e.g. "*.go") match files in any subdirectory
		if !strings.Contains(pattern, "/") && !isAnchored {
			return true
		}

		// Anchored Patterns
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

		if shouldIgnore(root, info.IsDir(), effectiveRules, false) {
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

		isIgnored := shouldIgnore(childPath, d.IsDir(), effectiveRules, parentIgnored)

		if d.IsDir() {
			if isIgnored {
				if hasNestedException(childPath, effectiveRules) {
					if err := w.recursiveWalk(fsys, childPath, effectiveRules, walkFn, true); err != nil {
						return err
					}
					continue
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
			rules = append(rules, parseRules(lines, dir)...)
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
