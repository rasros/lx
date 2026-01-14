package lx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// newTestRunner simplifies creating a runner for unit tests
func newTestRunner(head, tail int, tmplStr string) *Runner {
	if tmplStr == "" {
		tmplStr = defaultTemplate
	}

	funcs := templateFuncs()
	tMain := template.Must(template.New("test").Funcs(funcs).Parse(tmplStr))
	tSection := template.Must(template.New("section").Funcs(funcs).Parse(defaultSectionTemplate))
	tPrompt := template.Must(template.New("prompt").Funcs(funcs).Parse(defaultPromptTemplate))

	engine := &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
	}

	cfg := RunnerConfig{
		Head:        head,
		Tail:        tail,
		LineNumbers: false,
	}

	global := GlobalContext{
		TotalFiles:    1,
		TotalSize:     1024,
		TokenEstimate: 256,
		TotalSections: 1,
		WorkDir:       ".",
		Metadata:      make(map[string]string),
	}

	return NewRunner(cfg, engine, global)
}

func mustInputFile(t *testing.T, path string) InputFile {
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	abs, _ := filepath.Abs(path)
	return NewOsInputFile(path, abs, info)
}

func TestRunner_DefaultTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := newTestRunner(-1, -1, "")

	// Fix: Changed from r.RunFile(..., &buf) to receiving RenderedItem
	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "test.txt (3 rows)") {
		t.Errorf("missing default header info, got:\n%s", item.Body)
	}
	if !strings.Contains(item.Body, content) {
		t.Errorf("missing content")
	}
}

func TestRunner_CompactModeViaZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := newTestRunner(0, 0, "")

	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "rows)") {
		t.Errorf("Expected row count in compact mode, got:\n%s", item.Body)
	}
	if strings.Contains(item.Body, "```") {
		t.Errorf("Should not contain code block in compact mode")
	}
	if !item.IsCompactView {
		t.Errorf("Expected IsCompactView to be true")
	}
}

func TestRunner_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	r := newTestRunner(-1, -1, "")

	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "empty.txt - empty file") {
		t.Errorf("Expected 'empty file' notice, got:\n%s", item.Body)
	}
}

func TestRunner_CustomTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := "line1\nline2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl := "START {{ .Path }}\n{{ .Content }}END"
	r := newTestRunner(-1, -1, tmpl)

	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "START "+path) {
		t.Errorf("template header incorrect, got:\n%s", item.Body)
	}
	if !strings.Contains(item.Body, "line1\nline2\n") {
		t.Errorf("missing content")
	}
	if !strings.HasSuffix(strings.TrimSpace(item.Body), "END") {
		t.Errorf("missing template footer")
	}
}

func TestRunner_HeadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := newTestRunner(2, 0, "")

	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "a\nb\n") {
		t.Errorf("missing first two lines, got:\n%s", item.Body)
	}
	if strings.Contains(item.Body, "c\n") {
		t.Errorf("unexpected extra line")
	}
}

func TestRunner_BinaryDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.dat")
	content := []byte{0x00, 0x01, 0x02}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	tmpl := "{{ if .IsBinary }}BINARY{{ else }}TEXT{{ end }}"
	r := newTestRunner(-1, -1, tmpl)

	item, err := r.RunFile(mustInputFile(t, path), 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	if item.Body != "BINARY" {
		t.Errorf("Failed to detect binary file, got: %s", item.Body)
	}
}

func TestRunner_RunFile_Indexing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	r := newTestRunner(-1, -1, "")
	r.Global.TotalFiles = 100

	item, err := r.RunFile(mustInputFile(t, path), 42, 1)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	if !strings.Contains(item.Body, "[42/100]") {
		t.Errorf("Output missing file index [42/100], got:\n%s", item.Body)
	}
}
