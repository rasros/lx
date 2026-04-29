package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var defaultPromptExtensions = []string{".md", ".txt", ".prompt"}

type promptResolver struct {
	libDir     string
	extensions []string
}

func newPromptResolver(libDir string, extensions []string) *promptResolver {
	exts := extensions
	if len(exts) == 0 {
		exts = defaultPromptExtensions
	}
	return &promptResolver{libDir: libDir, extensions: exts}
}

func looksLikePath(v string) bool {
	if v == "" {
		return false
	}
	if filepath.IsAbs(v) {
		return true
	}
	if strings.HasPrefix(v, "~") {
		return true
	}
	if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
		return true
	}
	if strings.HasPrefix(v, ".\\") || strings.HasPrefix(v, "..\\") {
		return true
	}
	return false
}

func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func (r *promptResolver) resolve(value string) (string, error) {
	if looksLikePath(value) {
		path := expandHome(value)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("prompt file %q not found: %w", value, err)
		}
		return path, nil
	}

	if r.libDir == "" {
		return "", fmt.Errorf("prompt %q not found: no prompts library configured (set --prompts-dir, $LX_PROMPTS_DIR, or prompts_dir in config)", value)
	}

	libDir := expandHome(r.libDir)
	if info, err := os.Stat(libDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("prompts library %q is not a readable directory", libDir)
	}

	direct := filepath.Join(libDir, value)
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return direct, nil
	}

	for _, ext := range r.extensions {
		probe := filepath.Join(libDir, value+ext)
		if info, err := os.Stat(probe); err == nil && !info.IsDir() {
			return probe, nil
		}
	}

	matches, err := r.searchLib(libDir, value)
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		all, _ := r.listLib(libDir)
		return "", fmt.Errorf("prompt %q not found in %s%s", value, libDir, suggestList(all, value))
	default:
		rels := make([]string, 0, len(matches))
		for _, m := range matches {
			if rel, err := filepath.Rel(libDir, m); err == nil {
				rels = append(rels, rel)
			} else {
				rels = append(rels, m)
			}
		}
		sort.Strings(rels)
		return "", fmt.Errorf("prompt %q is ambiguous in %s; candidates:\n  %s", value, libDir, strings.Join(rels, "\n  "))
	}
}

func (r *promptResolver) searchLib(libDir, value string) ([]string, error) {
	var matches []string
	target := filepath.ToSlash(value)
	err := filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != libDir && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !r.hasAllowedExt(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(libDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if stripExt(relSlash) == target {
			matches = append(matches, path)
			return nil
		}
		base := filepath.Base(relSlash)
		if stripExt(base) == target {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func (r *promptResolver) listLib(libDir string) ([]string, error) {
	var entries []string
	err := filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != libDir && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !r.hasAllowedExt(d.Name()) {
			return nil
		}
		if rel, err := filepath.Rel(libDir, path); err == nil {
			entries = append(entries, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(entries)
	return entries, err
}

func (r *promptResolver) hasAllowedExt(name string) bool {
	if len(r.extensions) == 0 {
		return true
	}
	lower := strings.ToLower(name)
	for _, ext := range r.extensions {
		if ext == "" {
			if !strings.Contains(lower, ".") {
				return true
			}
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func stripExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return strings.TrimSuffix(name, ext)
}

func suggestList(entries []string, _ string) string {
	if len(entries) == 0 {
		return ""
	}
	max := 10
	if len(entries) < max {
		max = len(entries)
	}
	var sb strings.Builder
	sb.WriteString("\navailable prompts:\n  ")
	sb.WriteString(strings.Join(entries[:max], "\n  "))
	if len(entries) > max {
		sb.WriteString(fmt.Sprintf("\n  ... (%d more)", len(entries)-max))
	}
	return sb.String()
}

func resolvePromptsDir(parsed *ParsedArgs, cliCfg *CliConfig) string {
	if v, ok := parsed.Globals["prompts-dir"]; ok && v != "" {
		return v
	}
	if v := os.Getenv("LX_PROMPTS_DIR"); v != "" {
		return v
	}
	if cliCfg != nil && cliCfg.PromptsDir != "" {
		return cliCfg.PromptsDir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "lx", "prompts")
	}
	return ""
}

func runListPrompts(parsed *ParsedArgs) error {
	_, cliCfg, err := LoadConfigChain(parsed.Globals["config"])
	if err != nil {
		return err
	}
	dir := resolvePromptsDir(parsed, cliCfg)
	if dir == "" {
		return fmt.Errorf("no prompts library configured")
	}
	dir = expandHome(dir)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("prompts library %q is not a readable directory", dir)
	}
	exts := defaultPromptExtensions
	if cliCfg != nil && len(cliCfg.PromptExtensions) > 0 {
		exts = cliCfg.PromptExtensions
	}
	r := newPromptResolver(dir, exts)
	entries, err := r.listLib(dir)
	if err != nil {
		return err
	}
	for _, rel := range entries {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		fmt.Printf("%s\t%s\n", rel, abs)
	}
	return nil
}
