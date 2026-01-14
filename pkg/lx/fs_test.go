package lx

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStdinInputFile(t *testing.T) {
	content := []byte("hello stdin")
	f := NewBufferInputFile("stdin", content)

	if f.Path != "stdin" {
		t.Errorf("Path = %q, want stdin", f.Path)
	}
	if f.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", f.Size, len(content))
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != string(content) {
		t.Errorf("Content mismatch: %q", string(got))
	}
}

func TestOsInputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("data"), 0644)
	info, _ := os.Stat(path)

	f := NewOsInputFile(path, path, info)
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != "data" {
		t.Errorf("Read content = %q, want data", string(got))
	}
}
