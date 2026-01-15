package lx

import (
	"io"
	"io/fs"
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

func TestInputFile_DirFS(t *testing.T) {
	// Setup generic OS temp dir for the test environment
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fullPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	fsys := os.DirFS(dir)

	info, err := fs.Stat(fsys, "test.txt")
	if err != nil {
		t.Fatal(err)
	}

	f := NewInputFile(fsys, "test.txt", info)

	if f.Path != "test.txt" {
		t.Errorf("Path = %q, want test.txt", f.Path)
	}

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
