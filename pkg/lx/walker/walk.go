package walker

import (
	"bufio"
	"bytes"
	"io/fs"
	"path"
	"strings"
)

func (w *Walker) Walk(fsys fs.FS, root string, walkFn fs.WalkDirFunc) error {
	info, err := fs.Stat(fsys, root)
	if err != nil {
		return walkFn(root, nil, err)
	}

	if !info.IsDir() {
		var localRules []Rule
		if w.IgnoreEnabled {
			dir := path.Dir(root)
			// Always load root rules.
			localRules = append(localRules, w.loadIgnoreFiles(fsys, ".")...)

			if dir != "." {
				parts := strings.Split(dir, "/")
				curr := ""
				for _, p := range parts {
					if curr == "" {
						curr = p
					} else {
						curr = curr + "/" + p
					}
					localRules = append(localRules, w.loadIgnoreFiles(fsys, curr)...)
				}
			}
		}

		effectiveRules := make([]Rule, 0, len(w.BaseRules)+len(localRules)+len(w.OverrideRules))
		effectiveRules = append(effectiveRules, w.BaseRules...)
		effectiveRules = append(effectiveRules, localRules...)
		effectiveRules = append(effectiveRules, w.OverrideRules...)

		isIgnored, reason := checkIgnore(root, info.IsDir(), effectiveRules, false)
		if isIgnored {
			if w.OnIgnore != nil {
				w.OnIgnore(root, reason)
			}
			return nil
		}
		return walkFn(root, dirEntryAdapter{info}, nil)
	}

	initialRules := make([]Rule, 0, len(w.BaseRules)+len(w.OverrideRules))
	initialRules = append(initialRules, w.BaseRules...)
	initialRules = append(initialRules, w.OverrideRules...)
	return w.recursiveWalk(fsys, root, initialRules, walkFn, false)
}

func (w *Walker) recursiveWalk(fsys fs.FS, dir string, parentRules []Rule, walkFn fs.WalkDirFunc, parentIgnored bool) error {
	var localRules []Rule
	if w.IgnoreEnabled {
		localRules = w.loadIgnoreFiles(fsys, dir)
	}

	var effectiveRules []Rule
	if len(localRules) == 0 {
		effectiveRules = parentRules
	} else {
		effectiveRules = make([]Rule, 0, len(parentRules)+len(localRules)+len(w.OverrideRules))
		effectiveRules = append(effectiveRules, parentRules...)
		effectiveRules = append(effectiveRules, localRules...)
		effectiveRules = append(effectiveRules, w.OverrideRules...)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return walkFn(dir, nil, err)
	}

	for _, d := range entries {
		childPath := d.Name()
		if dir != "." {
			childPath = path.Join(dir, d.Name())
		}

		isIgnored, reason := checkIgnore(childPath, d.IsDir(), effectiveRules, parentIgnored)

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
			continue
		}

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
