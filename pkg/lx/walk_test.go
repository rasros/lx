package lx

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupWalkTestDir(t *testing.T) string {
	dir := t.TempDir()

	// Structure:
	// /file.txt
	// /ignore.txt
	// /.hidden
	// /.gitignore (ignores ignore.txt)
	// /sub/
	//    /subfile.go
	//    /link.txt -> ../file.txt

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignore.txt"), 0644); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "subfile.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "file.txt"), filepath.Join(sub, "link.txt")); err != nil {
		t.Fatal(err)
	}

	return dir
}

func collectPaths(ch <-chan InputFile) []string {
	var paths []string
	for f := range ch {
		paths = append(paths, filepath.Base(f.Path))
	}
	sort.Strings(paths)
	return paths
}

func TestWalker_Defaults(t *testing.T) {
	// Defaults: Ignore yes, Hidden no, Symlinks no
	dir := setupWalkTestDir(t)
	cfg := Config{}
	w := NewWalker(cfg)

	ch := w.Walk([]string{dir})
	got := collectPaths(ch)

	// Expect: file.txt, subfile.go
	// "ignore.txt" skipped by gitignore
	// ".hidden" AND ".gitignore" skipped by hidden check
	// "link.txt" skipped by default symlink check
	want := []string{"file.txt", "subfile.go"}

	if len(got) != len(want) {
		t.Fatalf("Got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Index %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

func TestWalker_NoIgnore(t *testing.T) {
	dir := setupWalkTestDir(t)
	f := false
	cfg := Config{Ignore: &f}
	w := NewWalker(cfg)

	ch := w.Walk([]string{dir})
	got := collectPaths(ch)

	// Expect: ignore.txt included
	mustHave(t, got, "ignore.txt")
}

func TestWalker_ShowHidden(t *testing.T) {
	dir := setupWalkTestDir(t)
	cfg := Config{ShowHidden: true}
	w := NewWalker(cfg)

	ch := w.Walk([]string{dir})
	got := collectPaths(ch)

	// Expect: .hidden and .gitignore included
	mustHave(t, got, ".hidden")
	mustHave(t, got, ".gitignore")
}

func TestWalker_FollowSymlinks(t *testing.T) {
	dir := setupWalkTestDir(t)
	cfg := Config{FollowSymlinks: true}
	w := NewWalker(cfg)

	ch := w.Walk([]string{dir})
	got := collectPaths(ch)

	// Expect: link.txt included
	mustHave(t, got, "link.txt")
}

func TestWalker_ExplicitMissing(t *testing.T) {
	cfg := Config{}
	w := NewWalker(cfg)
	ch := w.Walk([]string{"/non/existent/path"})

	f := <-ch
	if f.LoadError == nil {
		t.Error("Expected LoadError for missing explicit path")
	}
	if f.Path != "/non/existent/path" {
		t.Errorf("Expected path passed back, got %s", f.Path)
	}
}

func mustHave(t *testing.T, list []string, item string) {
	t.Helper()
	for _, x := range list {
		if x == item {
			return
		}
	}
	t.Errorf("List %v missing expected item %q", list, item)
}
