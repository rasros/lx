package lx

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalker_Walk(t *testing.T) {
	// Create a temporary directory tree
	// root/
	//    a.txt
	//    b.go
	//    sub/
	//      c.md
	//    .hidden
	//    ignore_me.txt
	tmpDir := t.TempDir()

	createFile(t, tmpDir, "a.txt")
	createFile(t, tmpDir, "b.go")
	createFile(t, tmpDir, "ignore_me.txt")
	createFile(t, tmpDir, ".hidden")
	mustMkdir(t, filepath.Join(tmpDir, "sub"))
	createFile(t, tmpDir, "sub/c.md")

	// Create a .gitignore in root
	ignoreContent := "ignore_me.txt\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(ignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Walks recursively and respects ignore", func(t *testing.T) {
		cfg := Config{
			ShowHidden: false,
			// Ignore is nil by default, which means enabled
		}
		w := NewWalker(cfg)

		ctx := context.Background()
		ch := w.Walk(ctx, []string{tmpDir})

		var paths []string
		for f := range ch {
			if f.LoadError != nil {
				t.Errorf("load error: %v", f.LoadError)
			}
			rel, _ := filepath.Rel(tmpDir, f.Path)
			paths = append(paths, rel)
		}
		sort.Strings(paths)

		expected := []string{"a.txt", "b.go", filepath.Join("sub", "c.md")}
		sort.Strings(expected)

		if !equal(paths, expected) {
			t.Errorf("expected %v, got %v", expected, paths)
		}
	})

	t.Run("Shows hidden files when Configured", func(t *testing.T) {
		cfg := Config{
			ShowHidden: true,
		}
		w := NewWalker(cfg)

		ctx := context.Background()
		ch := w.Walk(ctx, []string{tmpDir})

		foundHidden := false
		foundGitIgnore := false

		for f := range ch {
			rel, _ := filepath.Rel(tmpDir, f.Path)
			if rel == ".hidden" {
				foundHidden = true
			}
			if rel == ".gitignore" {
				foundGitIgnore = true
			}
		}

		if !foundHidden {
			t.Error("expected to find .hidden file")
		}
		if !foundGitIgnore {
			t.Error("expected to find .gitignore file")
		}
	})

	t.Run("Context Cancellation stops walk", func(t *testing.T) {
		cfg := Config{}
		w := NewWalker(cfg)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ch := w.Walk(ctx, []string{tmpDir})

		count := 0
		for range ch {
			count++
		}

		// It should return nearly 0 files.
		// Note: The race between cancel and goroutine start means it might get 1,
		// but usually 0 if cancelled immediately.
		if count > 2 {
			t.Errorf("expected walk to abort early, got %d files", count)
		}
	})
}

// Helper functions

func createFile(t *testing.T, dir, name string) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
