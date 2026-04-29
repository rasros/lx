package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "refactor.md"), "REFACTOR")
	writeFile(t, filepath.Join(dir, "go", "test.md"), "GO_TEST")
	writeFile(t, filepath.Join(dir, "python", "test.md"), "PY_TEST")
	writeFile(t, filepath.Join(dir, "claude", "code", "review.md"), "CC_REVIEW")
	writeFile(t, filepath.Join(dir, "notes.txt"), "NOTES")
	writeFile(t, filepath.Join(dir, "raw.prompt"), "RAW")
	return dir
}

func TestPromptResolver_FlatExactExtension(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("refactor")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "refactor.md") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_NestedRelativePath(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("go/test")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "go", "test.md") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_DeeplyNested(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("claude/code/review")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "claude", "code", "review.md") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_ExactWithExtension(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("go/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "go", "test.md") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_TxtExtension(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("notes")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "notes.txt") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_PromptExtension(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("raw")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "raw.prompt") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_AmbiguousBasename(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	_, err := r.resolve("test")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got %q", msg)
	}
	if !strings.Contains(msg, "go/test.md") || !strings.Contains(msg, "python/test.md") {
		t.Errorf("expected both candidates listed, got %q", msg)
	}
}

func TestPromptResolver_NotFound(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	_, err := r.resolve("nope/missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v", err)
	}
}

func TestPromptResolver_AbsolutePathBypassesLib(t *testing.T) {
	tmp := t.TempDir()
	external := filepath.Join(tmp, "external.md")
	writeFile(t, external, "EXTERNAL")

	r := newPromptResolver(t.TempDir(), defaultPromptExtensions)
	got, err := r.resolve(external)
	if err != nil {
		t.Fatal(err)
	}
	if got != external {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_RelativeDotPathBypassesLib(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	work := t.TempDir()
	writeFile(t, filepath.Join(work, "local.md"), "LOCAL")
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}

	r := newPromptResolver(t.TempDir(), defaultPromptExtensions)
	got, err := r.resolve("./local.md")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "local.md" {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_ExactRelativeBeatsBasename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "shared.md"), "A")
	writeFile(t, filepath.Join(dir, "b", "shared.md"), "B")

	r := newPromptResolver(dir, defaultPromptExtensions)
	got, err := r.resolve("a/shared")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "a", "shared.md") {
		t.Errorf("got %s", got)
	}
}

func TestPromptResolver_NoLibConfigured(t *testing.T) {
	r := newPromptResolver("", defaultPromptExtensions)
	_, err := r.resolve("anything")
	if err == nil {
		t.Fatal("expected error when libDir is empty")
	}
}

func TestPromptResolver_ListLib(t *testing.T) {
	dir := newTestLib(t)
	r := newPromptResolver(dir, defaultPromptExtensions)
	entries, err := r.listLib(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"claude/code/review.md",
		"go/test.md",
		"notes.txt",
		"python/test.md",
		"raw.prompt",
		"refactor.md",
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(entries), len(want), entries)
	}
	for i, e := range entries {
		if e != want[i] {
			t.Errorf("entry %d: got %q want %q", i, e, want[i])
		}
	}
}

func TestResolvePromptsDir_PrecedenceCLI(t *testing.T) {
	t.Setenv("LX_PROMPTS_DIR", "/from/env")
	parsed := &ParsedArgs{Globals: map[string]string{"prompts-dir": "/from/cli"}}
	cfg := &CliConfig{PromptsDir: "/from/cfg"}
	if got := resolvePromptsDir(parsed, cfg); got != "/from/cli" {
		t.Errorf("got %s", got)
	}
}

func TestResolvePromptsDir_PrecedenceEnvOverConfig(t *testing.T) {
	t.Setenv("LX_PROMPTS_DIR", "/from/env")
	parsed := &ParsedArgs{Globals: map[string]string{}}
	cfg := &CliConfig{PromptsDir: "/from/cfg"}
	if got := resolvePromptsDir(parsed, cfg); got != "/from/env" {
		t.Errorf("got %s", got)
	}
}

func TestResolvePromptsDir_ConfigOverDefault(t *testing.T) {
	t.Setenv("LX_PROMPTS_DIR", "")
	parsed := &ParsedArgs{Globals: map[string]string{}}
	cfg := &CliConfig{PromptsDir: "/from/cfg"}
	if got := resolvePromptsDir(parsed, cfg); got != "/from/cfg" {
		t.Errorf("got %s", got)
	}
}
