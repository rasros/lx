package lx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunner_DefaultDelimitersAndPlaceholders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := NewRunner(0, 0, "", "", false, false)

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, path+" (3 rows)\n---\n```text\n") {
		t.Errorf("missing default prefix with placeholders, got:\n%s", out)
	}
	if !strings.Contains(out, content) {
		t.Errorf("missing content")
	}
	if !strings.HasSuffix(out, "```\n\n") {
		t.Errorf("missing default postfix")
	}
}

func TestRunner_CustomDelimiters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := "line1\nline2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := Runner{
		PrefixDelimiter:  "BEGIN {filename} {row_count}\n",
		PostfixDelimiter: "END\n",
	}

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "BEGIN "+path+" 2\n") {
		t.Errorf("prefix substitution incorrect, got:\n%s", out)
	}
	if !strings.Contains(out, content) {
		t.Errorf("missing content")
	}
	if !strings.HasSuffix(out, "END\n") {
		t.Errorf("missing custom postfix")
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
	r := Runner{
		Head:             2,
		PrefixDelimiter:  "P\n",
		PostfixDelimiter: "Q\n",
	}

	if err := r.Run([]string{path}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	mid := strings.TrimPrefix(out, "P\n")
	mid = strings.TrimSuffix(mid, "Q\n")

	if !strings.Contains(mid, "a\nb\n") {
		t.Errorf("missing first two lines")
	}
	if strings.Contains(mid, "c\n") {
		t.Errorf("unexpected extra line")
	}
}

func TestRunner_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "f1.txt")
	p2 := filepath.Join(dir, "f2.txt")
	if err := os.WriteFile(p1, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	r := NewRunner(0, 0, "", "", false, false)

	if err := r.Run([]string{p1, p2}, &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, p1) || !strings.Contains(out, p2) {
		t.Errorf("missing prefixes for multiple files")
	}
	if !strings.Contains(out, "x\n") || !strings.Contains(out, "y\n") {
		t.Errorf("missing contents for multiple files")
	}
}

func TestRunner_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	r := NewRunner(0, 0, "", "", false, false)

	err := r.Run([]string{"no_such_file.txt"}, &buf)
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "no_such_file.txt") {
		t.Errorf("error does not mention filename: %v", err)
	}
}
