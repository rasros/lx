package walker

import (
	"io/fs"
	"os"
	"path/filepath"
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

	w := NewWalker(
		[]string{"*.log"},
		[]string{"!nested/important.log"},
	)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "root.log")
	assertNotContains(t, files, "nested/other.log")
	assertContains(t, files, "nested/important.log")
}

func TestWalker_WildcardException(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "logs/app")
	mustMkdir(t, tmp, "logs/db")

	mustWriteFile(t, tmp, "logs/app/junk.txt", "trash")
	mustWriteFile(t, tmp, "logs/app/keep.txt", "treasure")
	mustWriteFile(t, tmp, "logs/db/junk.txt", "trash")
	mustWriteFile(t, tmp, "logs/db/keep.txt", "treasure")

	w := NewWalker(
		[]string{"logs"},
		[]string{"!logs/*/keep.txt"},
	)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "logs/app/junk.txt")
	assertNotContains(t, files, "logs/db/junk.txt")
	assertContains(t, files, "logs/app/keep.txt")
	assertContains(t, files, "logs/db/keep.txt")
}

func TestWalker_StartOnFile(t *testing.T) {
	tmp := t.TempDir()
	mustWriteFile(t, tmp, "hello.txt", "world")

	w := NewWalker(nil, nil)

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
	mustWriteFile(t, tmp, "ignore.me", "content")
	mustWriteFile(t, tmp, ".gitignore", "*.me")

	w := NewWalker(nil, nil)

	var paths []string
	fsys := os.DirFS(tmp)

	err := w.Walk(fsys, "ignore.me", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("Expected file to be ignored by .gitignore, but got: %v", paths)
	}
}

func TestWalker_SkipHidden(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, ".hidden")
	mustMkdir(t, tmp, "visible")
	mustWriteFile(t, tmp, ".env", "secret")
	mustWriteFile(t, tmp, ".hidden/file.txt", "secret")
	mustWriteFile(t, tmp, "visible/file.txt", "public")

	w := NewWalker(nil, nil)
	w.SkipHidden = true
	w.IgnoreEnabled = false

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

	w := NewWalker([]string{"/foo.txt"}, nil)

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

	w := NewWalker([]string{"**/junk.log"}, nil)

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
	w1 := NewWalker(
		[]string{"file.txt"},
		[]string{"!file.txt"},
	)
	files1 := collectPaths(t, w1, tmp)
	assertContains(t, files1, "file.txt")

	// Same level: Last wins
	w2 := NewWalker(
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

	w := NewWalker([]string{"bin/"}, nil)

	files := collectPaths(t, w, tmp)

	assertNotContains(t, files, "bin/exec")
	assertContains(t, files, "src/code.go")
}

func TestWalker_LoadLocalGitignore(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "nested")

	mustWriteFile(t, tmp, "root.txt", "root")
	mustWriteFile(t, tmp, "nested/should_ignore.txt", "bye")
	mustWriteFile(t, tmp, "nested/should_keep.txt", "hi")
	mustWriteFile(t, tmp, "nested/.gitignore", "should_ignore.txt")

	w := NewWalker(nil, nil)
	files := collectPaths(t, w, tmp)

	assertContains(t, files, "root.txt")
	assertContains(t, files, "nested/should_keep.txt")
	assertNotContains(t, files, "nested/should_ignore.txt")
}

func TestWalker_NestedGitignoreOverride(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, tmp, "deep")

	mustWriteFile(t, tmp, ".gitignore", "*\n!deep/")
	mustWriteFile(t, tmp, "deep/.gitignore", "!target.txt")

	mustWriteFile(t, tmp, "root_file.txt", "ignored")
	mustWriteFile(t, tmp, "deep/target.txt", "kept")
	mustWriteFile(t, tmp, "deep/other.txt", "ignored")

	w := NewWalker(nil, nil)
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

func TestIsMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Basename / floating patterns match at any depth
		{"*.go", "main.go", true},
		{"*.go", "sub/main.go", true},
		{"*.go", "main.txt", false},
		// Anchored pattern only matches at root
		{"/foo.txt", "foo.txt", true},
		{"/foo.txt", "sub/foo.txt", false},
		// Path pattern (contains slash) is matched against full path
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "pkg/main.go", false},
		// Double star spans directories
		{"**/test", "a/b/test", true},
		{"**/test", "test", true},
		{"**/test", "a/b/other", false},
		// Literal floating name matches any segment
		{"target", "target", true},
		{"target", "src/target", true},
		{"target", "other", false},
		// Dir-only pattern: last segment of a plain path is excluded from matching
		{"node_modules/", "node_modules/pkg/foo.go", true},
		{"node_modules/", "other/foo.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"|"+tt.path, func(t *testing.T) {
			got := IsMatch(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("IsMatch(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
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

	t.Run("IsLiteral set correctly", func(t *testing.T) {
		rules := parseRules([]string{"foo", "*.go", "bar/baz", "a[bc]"}, "", "test")
		cases := []struct {
			idx  int
			want bool
			name string
		}{
			{0, true, "foo"},
			{1, false, "*.go"},
			{2, true, "bar/baz"},
			{3, false, "a[bc]"},
		}
		for _, c := range cases {
			if rules[c.idx].Spec.Pattern.IsLiteral != c.want {
				t.Errorf("rules[%d] (%s).Spec.Pattern.IsLiteral = %v, want %v", c.idx, c.name, rules[c.idx].Spec.Pattern.IsLiteral, c.want)
			}
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

			w := NewWalker(nil, nil)
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

	w := NewWalker([]string{"*.log"}, nil)
	var ignored []string
	w.OnIgnore = func(path string, reason string) {
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

	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	files := collectPaths(t, w, tmp)
	assertContains(t, files, "keep.log")
}
