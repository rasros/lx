package lx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// newTestRunner is a helper to create a Runner with compiled templates.
func newTestRunner(head, tail int, tmplStr, sectionTmplStr string) *Runner {
	if tmplStr == "" {
		tmplStr = DefaultTemplate
	}
	if sectionTmplStr == "" {
		sectionTmplStr = DefaultSectionTemplate
	}

	t := template.Must(template.New("test").Funcs(TemplateFuncs()).Parse(tmplStr))
	st := template.Must(template.New("section").Funcs(TemplateFuncs()).Parse(sectionTmplStr))

	return NewRunner(head, tail, t, st, false)
}

func TestRunner_DefaultTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := newTestRunner(0, 0, "", "")

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "test.txt (3 rows)") {
		t.Errorf("missing default header info, got:\n%s", out)
	}
	if !strings.Contains(out, content) {
		t.Errorf("missing content")
	}
}

func TestRunner_CustomTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := "line1\nline2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	tmpl := "START {{ .Path }}\n{{ .Content }}END"
	r := newTestRunner(0, 0, tmpl, "")

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "START "+path) {
		t.Errorf("template header incorrect, got:\n%s", out)
	}
	if !strings.Contains(out, "line1\nline2\n") {
		t.Errorf("missing content")
	}
	if !strings.HasSuffix(out, "END") {
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

	var buf bytes.Buffer
	r := newTestRunner(2, 0, "{{ .Content }}", "")

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "a\nb\n") {
		t.Errorf("missing first two lines")
	}
	if strings.Contains(out, "c\n") {
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

	var buf bytes.Buffer
	tmpl := "{{ if .IsBinary }}BINARY{{ else }}TEXT{{ end }}"
	r := newTestRunner(0, 0, tmpl, "")

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != "BINARY" {
		t.Errorf("Failed to detect binary file, got: %s", buf.String())
	}
}

func TestRunner_RunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := newTestRunner(0, 0, "", "") // default templates

	err := r.RunFile(path, 42, 100, &buf)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[42/100]") {
		t.Errorf("Output missing file index [42/100], got:\n%s", out)
	}
}

func TestRunner_RunSection(t *testing.T) {
	var buf bytes.Buffer

	// Define a custom section template
	sectionTmpl := ">>> {{ .Name }} <<<"

	r := newTestRunner(0, 0, "", sectionTmpl)

	if err := r.RunSection("Environment Setup", &buf); err != nil {
		t.Fatalf("RunSection error: %v", err)
	}

	// Expect two newlines at the end to form a blank row
	want := ">>> Environment Setup <<<\n\n"
	if buf.String() != want {
		t.Errorf("RunSection output = %q, want %q", buf.String(), want)
	}
}

func TestRunner_RunPrompt_AddsBlankRow(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(0, 0, "", "")

	// Case 1: No newline provided -> adds \n\n
	if err := r.RunPrompt("Hello", &buf); err != nil {
		t.Fatalf("RunPrompt error: %v", err)
	}
	if buf.String() != "Hello\n\n" {
		t.Errorf("RunPrompt(Hello) = %q, want %q", buf.String(), "Hello\\n\\n")
	}

	buf.Reset()

	// Case 2: Newline already exists -> adds one \n to make it \n\n
	if err := r.RunPrompt("World\n", &buf); err != nil {
		t.Fatalf("RunPrompt error: %v", err)
	}
	if buf.String() != "World\n\n" {
		t.Errorf("RunPrompt(World\\n) = %q, want %q", buf.String(), "World\\n\\n")
	}

	buf.Reset()

	// Case 3: Two newlines exist -> leaves it alone
	if err := r.RunPrompt("Done\n\n", &buf); err != nil {
		t.Fatalf("RunPrompt error: %v", err)
	}
	if buf.String() != "Done\n\n" {
		t.Errorf("RunPrompt(Done\\n\\n) = %q, want %q", buf.String(), "Done\\n\\n")
	}
}
