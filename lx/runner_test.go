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
	t := template.Must(template.New("test").Funcs(TemplateFuncs()).Parse(tmplStr))
	return NewRunner(head, tail, t, false)
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

func TestRunner_BinaryDetection_PNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	// PNG Magic Bytes: 89 50 4E 47 0D 0A 1A 0A
	content := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
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
		t.Errorf("Failed to detect PNG as binary, got: %s", buf.String())
	}
}

func TestRunner_MultipleFiles_Indexing(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "f1.txt")
	p2 := filepath.Join(dir, "f2.txt")
	_ = os.WriteFile(p1, []byte("A"), 0644)
	_ = os.WriteFile(p2, []byte("B"), 0644)

	var buf bytes.Buffer
	r := newTestRunner(0, 0, "") // Use DefaultTemplate

	if err := r.Run([]string{p1, p2}, &buf); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	
	// Check for "[1/2]" and "[2/2]" in output
	if !strings.Contains(out, "[1/2]") {
		t.Errorf("Output missing first file index [1/2]")
	}
	if !strings.Contains(out, "[2/2]") {
		t.Errorf("Output missing second file index [2/2]")
	}
}
