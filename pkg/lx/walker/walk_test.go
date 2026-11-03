package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestWalker_NestedIgnores(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "nested")

	mustWriteFile(t, tmp, ".gitignore", "*.log")
	mustWriteFile(t, tmp, "root.log", "ignore me")
	mustWriteFile(t, tmp, "nested/other.log", "ignore me too")
	mustWriteFile(t, tmp, "nested/important.log", "keep me")

	w := newWalker(
		[]string{"*.log"},
		[]string{"!nested/important.log"},
	)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "root.log")
	assertNotContains(t, files, "nested/other.log")
	assertContains(t, files, "nested/important.log")
}

func TestWalker_StartOnFile(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, tmp, "hello.txt", "world")

	w := newWalker(nil, nil)

	var paths []string
	fsys := os.DirFS(tmp)

	err := w.Walk(fsys, "hello.txt", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(paths) != 1 || paths[0] != "hello.txt" {
		t.Errorf("Expected [hello.txt], got %v", paths)
	}
}

func TestWalker_StartOnFile_Ignored(t *testing.T) {
	tmp := t.TempDir()
	mustGitDir(t, tmp, ".git")
	mustWriteFile(t, tmp, "ignore.me", "content")
	mustWriteFile(t, tmp, ".gitignore", "*.me")

	// The file is named, so it is included; a walk skips it.
	if got := walkNamed(t, newWalker(nil, nil), tmp, "ignore.me"); !reflect.DeepEqual(got, []string{"ignore.me"}) {
		t.Errorf("expected the named file to be included, got %v", got)
	}
	assertNotContains(t, collectPaths(t, newWalker(nil, nil), tmp), "ignore.me")
}

func TestWalker_SkipHidden(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, ".hidden")
	mustMkdir(t, tmp, "visible")
	mustWriteFile(t, tmp, ".env", "secret")
	mustWriteFile(t, tmp, ".hidden/file.txt", "secret")
	mustWriteFile(t, tmp, "visible/file.txt", "public")

	w := newWalker(nil, nil)
	w.opts.SkipHidden = true
	w.opts.IgnoreEnabled = false

	files := collectPaths(t, w, tmp)
	assertNotContains(t, files, ".env")
	assertNotContains(t, files, ".hidden/file.txt")
	assertContains(t, files, "visible/file.txt")
}

func TestWalker_Anchoring(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "nested")

	mustWriteFile(t, tmp, "foo.txt", "root foo")
	mustWriteFile(t, tmp, "nested/foo.txt", "nested foo")

	w := newWalker([]string{"/foo.txt"}, nil)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "foo.txt")
	assertContains(t, files, "nested/foo.txt")
}

func TestWalker_DoubleStar(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "a/b/c")

	mustWriteFile(t, tmp, "a/junk.log", "trash")
	mustWriteFile(t, tmp, "a/b/junk.log", "trash")
	mustWriteFile(t, tmp, "a/b/c/junk.log", "trash")
	mustWriteFile(t, tmp, "a/b/c/keep.txt", "treasure")

	w := newWalker([]string{"**/junk.log"}, nil)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "a/junk.log")
	assertNotContains(t, files, "a/b/junk.log")
	assertNotContains(t, files, "a/b/c/junk.log")
	assertContains(t, files, "a/b/c/keep.txt")
}

func TestWalker_Precedence(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, tmp, "file.txt", "content")

	// Base vs Override: Override wins
	w1 := newWalker(
		[]string{"file.txt"},
		[]string{"!file.txt"},
	)
	files1 := collectPaths(t, w1, tmp)
	assertContains(t, files1, "file.txt")

	// Same level: Last wins
	w2 := newWalker(
		[]string{"!file.txt", "file.txt"},
		nil,
	)
	files2 := collectPaths(t, w2, tmp)
	assertNotContains(t, files2, "file.txt")
}

func TestWalker_DirectoryTrailingSlash(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "bin")
	mustMkdir(t, tmp, "src")

	mustWriteFile(t, tmp, "bin/exec", "binary")
	mustWriteFile(t, tmp, "src/code.go", "code")

	w := newWalker([]string{"bin/"}, nil)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "bin/exec")
	assertContains(t, files, "src/code.go")
}

func TestWalker_LoadLocalGitignore(t *testing.T) {
	tmp := t.TempDir()
	mustGitDir(t, tmp, ".git")
	mustMkdir(t, tmp, "nested")

	mustWriteFile(t, tmp, "root.txt", "root")
	mustWriteFile(t, tmp, "nested/should_ignore.txt", "bye")
	mustWriteFile(t, tmp, "nested/should_keep.txt", "hi")
	mustWriteFile(t, tmp, "nested/.gitignore", "should_ignore.txt")

	w := newWalker(nil, nil)
	files := collectPaths(t, w, tmp)

	assertContains(t, files, "root.txt")
	assertContains(t, files, "nested/should_keep.txt")
	assertNotContains(t, files, "nested/should_ignore.txt")
}

func TestWalker_NestedGitignoreOverride(t *testing.T) {
	tmp := t.TempDir()
	mustGitDir(t, tmp, ".git")
	mustMkdir(t, tmp, "deep")

	mustWriteFile(t, tmp, ".gitignore", "*\n!deep/")
	mustWriteFile(t, tmp, "deep/.gitignore", "!target.txt")

	mustWriteFile(t, tmp, "root_file.txt", "ignored")
	mustWriteFile(t, tmp, "deep/target.txt", "kept")
	mustWriteFile(t, tmp, "deep/other.txt", "ignored")

	w := newWalker(nil, nil)
	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "root_file.txt")
	assertNotContains(t, files, "deep/other.txt")
	assertContains(t, files, "deep/target.txt")
}

func collectPaths(t *testing.T, w *Walker, rootDir string) []string {
	var paths []string
	fsys := os.DirFS(rootDir)

	err := w.Walk(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == ".gitignore" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	sort.Strings(paths)
	return paths
}

func mustWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	fp := filepath.Join(dir, name)
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
		t.Fatal(err)
	}
}

// mustGitDir creates a minimal git directory at dir/name: a HEAD and the
// objects and refs directories git requires before it treats one as a repository.
func mustGitDir(t *testing.T, dir, name string) {
	t.Helper()
	mustMkdir(t, dir, filepath.Join(name, "objects"))
	mustMkdir(t, dir, filepath.Join(name, "refs"))
	mustWriteFile(t, dir, filepath.Join(name, "HEAD"), "ref: refs/heads/main\n")
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

func TestParseRules(t *testing.T) {
	t.Run("comment and blank lines skipped", func(t *testing.T) {
		rules := parseRules([]string{"# comment", "  ", "foo"}, "", "test")
		if len(rules) != 1 {
			t.Fatalf("Expected 1 rule, got %d: %v", len(rules), rules)
		}
		if rules[0].Pattern != "foo" {
			t.Errorf("Pattern = %q, want foo", rules[0].Pattern)
		}
	})

	t.Run("negate prefix stripped and flagged", func(t *testing.T) {
		rules := parseRules([]string{"!important.go"}, "", "test")
		if !rules[0].Negate {
			t.Error("Expected Negate=true")
		}
		if rules[0].Pattern != "important.go" {
			t.Errorf("Pattern = %q, want important.go", rules[0].Pattern)
		}
	})

	t.Run("trailing slash preserved after clean", func(t *testing.T) {
		rules := parseRules([]string{"build/"}, "", "test")
		if rules[0].Pattern != "build/" {
			t.Errorf("Pattern = %q, want build/", rules[0].Pattern)
		}
	})
}

func TestWalker_AlternateIgnoreFiles(t *testing.T) {
	for _, ignoreFile := range []string{".ignore", ".lxignore"} {
		t.Run(ignoreFile, func(t *testing.T) {
			tmp := t.TempDir()
			mustWriteFile(t, tmp, ignoreFile, "secret.txt")
			mustWriteFile(t, tmp, "secret.txt", "hidden")
			mustWriteFile(t, tmp, "public.txt", "visible")

			w := newWalker(nil, nil)
			files := collectPaths(t, w, tmp)

			assertNotContains(t, files, "secret.txt")
			assertContains(t, files, "public.txt")
		})
	}
}

func TestWalker_OnIgnore(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, tmp, "junk.log", "trash")
	mustWriteFile(t, tmp, "keep.go", "keep")

	w := newWalker([]string{"*.log"}, nil)
	var ignored []string
	w.opts.OnIgnore = func(path string, reason string) {
		ignored = append(ignored, path)
	}

	_ = collectPaths(t, w, tmp)

	found := false
	for _, p := range ignored {
		if p == "junk.log" {
			found = true
		}
	}
	if !found {
		t.Errorf("OnIgnore not called for junk.log; got: %v", ignored)
	}
}

func TestWalker_IgnoreDisabled(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, tmp, ".gitignore", "*.log")
	mustWriteFile(t, tmp, "keep.log", "content")

	w := newWalker(nil, nil)
	w.opts.IgnoreEnabled = false

	files := collectPaths(t, w, tmp)
	assertContains(t, files, "keep.log")
}

// walkNamed walks the single path name inside dir and returns the paths
// passed to the callback.
func walkNamed(t *testing.T, w *Walker, dir, name string) []string {
	t.Helper()
	var got []string
	err := w.Walk(os.DirFS(dir), name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		got = append(got, p)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
	return got
}

// patterns.
func newWalker(base, override []string) *Walker {
	var overrides []Override
	for _, p := range override {
		overrides = append(overrides, ExcludeOverride(p))
	}
	return New(Options{Base: base, Overrides: overrides, IgnoreEnabled: true})
}
