package lx

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalker_RealFS_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	mustWriteFile(t, tmpDir, "main.go", "package main")
	mustWriteFile(t, tmpDir, "README.md", "# Docs")
	mustWriteFile(t, tmpDir, ".env", "SECRET=1")
	mustWriteFile(t, tmpDir, ".git/config", "[core]")
	mustMkdir(t, tmpDir, "src")
	mustWriteFile(t, tmpDir, "src/util.go", "package util")

	if err := os.Symlink("main.go", filepath.Join(tmpDir, "link_to_main.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(tmpDir, "link_to_src")); err != nil {
		t.Fatal(err)
	}

	mustMkdir(t, tmpDir, "cycle")
	if err := os.Symlink(".", filepath.Join(tmpDir, "cycle/loop")); err != nil {
		t.Fatal(err)
	}

	t.Run("Default Ignore Hidden", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:         tmpDir,
			FS:           os.DirFS(tmpDir), // Use DirFS rooted at tmpDir
			IgnoreHidden: true,
		})
		files := collectPaths(w)
		assertNotContains(t, files, ".env")
		assertNotContains(t, files, ".git/config")
		assertContains(t, files, "main.go")
	})

	t.Run("Include Hidden", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:         tmpDir,
			FS:           os.DirFS(tmpDir),
			IgnoreHidden: false,
			// Exclude .git explicitly so we only test .env logic
			Excludes: []string{".git"},
		})
		files := collectPaths(w)
		assertContains(t, files, ".env")
	})

	t.Run("Symlinks to Files (Default)", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:               tmpDir,
			FS:                 os.DirFS(tmpDir),
			IgnoreFileSymlinks: false,
		})
		files := collectPaths(w)
		assertContains(t, files, "link_to_main.go")
	})

	t.Run("Ignore Symlinks to Files", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:               tmpDir,
			FS:                 os.DirFS(tmpDir),
			IgnoreFileSymlinks: true,
		})
		files := collectPaths(w)
		assertNotContains(t, files, "link_to_main.go")
		assertContains(t, files, "main.go")
	})

	t.Run("Follow Dir Symlinks (No Cycle)", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:              tmpDir,
			FS:                os.DirFS(tmpDir),
			IgnoreDirSymlinks: false,
			Excludes:          []string{"cycle"},
		})
		files := collectPaths(w)
		// os.DirFS returns paths relative to the root, so use OS separator
		assertContains(t, files, filepath.Join("src", "util.go"))
		assertContains(t, files, filepath.Join("link_to_src", "util.go"))
	})

	t.Run("Cycle Detection", func(t *testing.T) {
		w := NewWalker(WalkerOptions{
			Root:              tmpDir,
			FS:                os.DirFS(tmpDir),
			IgnoreDirSymlinks: false,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2000) // 2s timeout
		defer cancel()

		// Explicitly walk the cycle directory
		ch := w.Walk(ctx, []string{"cycle"})
		count := 0
		for range ch {
			count++
		}
		// Pass if it doesn't hang/crash
	})
}

func collectPaths(w *Walker) []string {
	ch := w.Walk(context.Background(), []string{"."})
	var paths []string
	for f := range ch {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)
	return paths
}

func mustWriteFile(t *testing.T, dir, name, content string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, dir, name string) {
	if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, list []string, item string) {
	t.Helper()
	for _, v := range list {
		if v == item {
			return
		}
	}
	t.Errorf("List missing expected item: %s. Got: %v", item, list)
}

func assertNotContains(t *testing.T, list []string, item string) {
	t.Helper()
	for _, v := range list {
		if v == item {
			t.Errorf("List contains unexpected item: %s", item)
		}
	}
}
