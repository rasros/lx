package lx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func newTestRunner(head, tail int, tmplStr string) *Runner {
	if tmplStr == "" {
		tmplStr = DefaultTemplate
	}

	funcs := TemplateFuncs()
	tMain := template.Must(template.New("test").Funcs(funcs).Parse(tmplStr))
	tSection := template.Must(template.New("section").Funcs(funcs).Parse(DefaultSectionTemplate))
	tPrompt := template.Must(template.New("prompt").Funcs(funcs).Parse(DefaultPromptTemplate))

	return NewRunner(head, tail, tMain, tSection, tPrompt, false)
}

func TestRunner_DefaultTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := newTestRunner(0, 0, "")

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
	r := newTestRunner(0, 0, tmpl)

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
	r := newTestRunner(2, 0, "{{ .Content }}")

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
	r := newTestRunner(0, 0, tmpl)

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
	r := newTestRunner(0, 0, "") // default template

	err := r.RunFile(path, 42, 100, &buf)
	if err != nil {
		t.Fatalf("RunFile error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[42/100]") {
		t.Errorf("Output missing file index [42/100], got:\n%s", out)
	}
}
