package cli

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var defaultPromptExtensions = []string{".md", ".txt", ".prompt"}

// promptResolver finds -P prompts. Paths are shown as given and resolved
// against env's working directory.
type promptResolver struct {
	env        Env
	libDir     string
	extensions []string
}

func newPromptResolver(env Env, libDir string, extensions []string) *promptResolver {
	exts := extensions
	if len(exts) == 0 {
		exts = defaultPromptExtensions
	}
	return &promptResolver{env: env, libDir: libDir, extensions: exts}
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

func expandHome(env Env, p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := env.homeDir()
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
		path := expandHome(r.env, value)
		if _, err := r.env.stat(path); err != nil {
			return "", fmt.Errorf("prompt file %q not found: %w", value, err)
		}
		return path, nil
	}

	if r.libDir == "" {
		return "", fmt.Errorf("prompt %q not found: no prompts library configured (set --prompts-dir, $LX_PROMPTS_DIR, or prompts_dir in config)", value)
	}

	libDir := expandHome(r.env, r.libDir)
	if info, err := r.env.stat(libDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("prompts library %q is not a readable directory", libDir)
	}

	direct := filepath.Join(libDir, value)
	if info, err := r.env.stat(direct); err == nil && !info.IsDir() {
		return direct, nil
	}

	for _, ext := range r.extensions {
		probe := filepath.Join(libDir, value+ext)
		if info, err := r.env.stat(probe); err == nil && !info.IsDir() {
			return probe, nil
		}
	}

	matches, err := r.searchLib(libDir, value)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		all, _ := r.listLib(libDir)
		return "", fmt.Errorf("prompt %q not found in %s%s", value, libDir, suggestList(all))
	}
	shallowest := pickShallowest(libDir, matches)
	if len(shallowest) == 1 {
		return shallowest[0], nil
	}
	rels := make([]string, 0, len(shallowest))
	for _, m := range shallowest {
		if rel, err := filepath.Rel(libDir, m); err == nil {
			rels = append(rels, filepath.ToSlash(rel))
		} else {
			rels = append(rels, m)
		}
	}
	sort.Strings(rels)
	return "", fmt.Errorf("prompt %q is ambiguous in %s; candidates:\n  %s", value, libDir, strings.Join(rels, "\n  "))
}

// searchLib returns the prompts in libDir named value, below libDir as given.
func (r *promptResolver) searchLib(libDir, value string) ([]string, error) {
	entries, err := r.listLib(libDir)
	target := filepath.ToSlash(value)
	var matches []string
	for _, rel := range entries {
		if stripExt(rel) == target || stripExt(path.Base(rel)) == target {
			matches = append(matches, filepath.Join(libDir, filepath.FromSlash(rel)))
		}
	}
	return matches, err
}

func pickShallowest(libDir string, matches []string) []string {
	if len(matches) <= 1 {
		return matches
	}
	minDepth := -1
	depths := make([]int, len(matches))
	for i, m := range matches {
		rel, err := filepath.Rel(libDir, m)
		if err != nil {
			rel = m
		}
		d := strings.Count(filepath.ToSlash(rel), "/")
		depths[i] = d
		if minDepth == -1 || d < minDepth {
			minDepth = d
		}
	}
	out := matches[:0:0]
	for i, m := range matches {
		if depths[i] == minDepth {
			out = append(out, m)
		}
	}
	return out
}

func (r *promptResolver) listLib(libDir string) ([]string, error) {
	var entries []string
	absLib := r.env.abs(libDir)
	err := filepath.WalkDir(absLib, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != absLib && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !r.hasAllowedExt(d.Name()) {
			return nil
		}
		if rel, err := filepath.Rel(absLib, path); err == nil {
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

func suggestList(entries []string) string {
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

func resolvePromptsDir(parsed *ParsedArgs, cliCfg *CliConfig, env Env) string {
	if v, ok := parsed.Globals["prompts-dir"]; ok && v != "" {
		return v
	}
	if v := env.getenv("LX_PROMPTS_DIR"); v != "" {
		return v
	}
	if cliCfg != nil && cliCfg.PromptsDir != "" {
		return cliCfg.PromptsDir
	}
	if home, err := env.homeDir(); err == nil {
		return filepath.Join(home, ".config", "lx", "prompts")
	}
	return ""
}

func runListPrompts(parsed *ParsedArgs, env Env) error {
	_, cliCfg, err := LoadConfigChain(env, newLogger(env, slog.LevelWarn), parsed.Globals["config"])
	if err != nil {
		return err
	}
	dir := resolvePromptsDir(parsed, cliCfg, env)
	if dir == "" {
		return fmt.Errorf("no prompts library configured")
	}
	dir = expandHome(env, dir)
	if info, err := env.stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("prompts library %q is not a readable directory", dir)
	}
	exts := defaultPromptExtensions
	if cliCfg != nil && len(cliCfg.PromptExtensions) > 0 {
		exts = cliCfg.PromptExtensions
	}
	r := newPromptResolver(env, dir, exts)
	entries, err := r.listLib(dir)
	if err != nil {
		return err
	}
	for _, rel := range entries {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		fmt.Fprintf(env.Stdout, "%s\t%s\n", rel, abs)
	}
	return nil
}
