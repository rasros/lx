package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func offsetRule(t *testing.T, pattern, offset string) Rule {
	t.Helper()
	rules := parseRules([]string{pattern}, ".", ".gitignore")
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	rules[0].PathOffset = offset
	return rules[0]
}

func TestMatch_PathOffsetAnchoredRule(t *testing.T) {
	rule := offsetRule(t, "/sub/skip.txt", "sub/")
	if !match(rule, "skip.txt", false) {
		t.Fatal("expected anchored ancestor rule to match through the offset")
	}
	if match(rule, "other/skip.txt", false) {
		t.Fatal("expected anchored ancestor rule not to match a different path")
	}
}

func TestMatch_PathOffsetFloatingRuleMatchesWalkRoot(t *testing.T) {
	rule := offsetRule(t, "dist", "dist/")
	if !match(rule, "out.js", false) {
		t.Fatal("expected a floating rule naming the walk root to match its contents, as in git")
	}
}

func TestWalk_SingleFileAncestorRuleMatchesParentDir(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "out.js", "x")

	w := NewWalker(nil, nil)
	w.AncestorRules = []Rule{offsetRule(t, "dist", "dist/")}
	if walked := walkFrom(t, w, dir, "out.js"); len(walked) != 0 {
		t.Fatalf("expected a file below an ignored parent to be ignored, got %v", walked)
	}
}

func TestWalk_RootIgnoredSkipsEverything(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "a.go", "package a")

	w := NewWalker(nil, nil)
	w.RootIgnored = true
	if walked := walkFrom(t, w, dir, "."); len(walked) != 0 {
		t.Fatalf("expected nothing below an ignored root, got %v", walked)
	}
	if walked := walkFrom(t, w, dir, "a.go"); len(walked) != 0 {
		t.Fatalf("expected a file in an ignored root to be ignored, got %v", walked)
	}

	w.IgnoreEnabled = false
	if walked := walkFrom(t, w, dir, "a.go"); len(walked) != 1 {
		t.Fatalf("expected disabled ignores to include the file, got %v", walked)
	}
}

func walkFrom(t *testing.T, w *Walker, dir, root string) []string {
	t.Helper()
	var walked []string
	err := w.Walk(os.DirFS(dir), root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			walked = append(walked, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return walked
}

func TestHasNestedException_PathOffset(t *testing.T) {
	rules := []Rule{offsetRule(t, "!/sub/build/keep.go", "sub/")}
	if !hasNestedException("build", rules) {
		t.Fatal("expected negation below build to be found through the offset")
	}
	if hasNestedException("other", rules) {
		t.Fatal("expected no exception for an unrelated directory")
	}
}

func TestLoadAncestorRules_StopsAtRepoRoot(t *testing.T) {
	outer := t.TempDir()
	mustWriteFile(t, outer, ".gitignore", "outer.txt\n")
	mustMkdir(t, outer, "repo/a/b")
	repo := filepath.Join(outer, "repo")
	mustWriteFile(t, repo, ".git", "gitdir: /elsewhere\n")
	mustWriteFile(t, repo, ".gitignore", "*.log\n")
	mustWriteFile(t, repo, "a/.ignore", "tmp/\n")

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(filepath.Join(repo, "a", "b"))

	got := map[string]string{}
	for _, r := range w.AncestorRules {
		got[r.Pattern] = r.PathOffset
	}
	want := map[string]string{"*.log": "a/b/", "tmp/": "b/"}
	if len(got) != len(want) {
		t.Fatalf("got rules %v, want %v", got, want)
	}
	for p, off := range want {
		if got[p] != off {
			t.Fatalf("rule %q: got offset %q, want %q", p, got[p], off)
		}
	}
}

func TestLoadAncestorRules_RootIgnored(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, repo, ".git")
	mustWriteFile(t, repo, ".gitignore", "/build\ndist\ngen/**\nout\nlib/*\n!lib/keep\n")
	for _, d := range []string{"build/x", "pkg/dist", "gen", "out/keep", "src/a", "lib/keep", "node_modules/pkg"} {
		mustMkdir(t, repo, d)
	}
	mustWriteFile(t, repo, "out/.gitignore", "!keep\n")

	tests := []struct {
		dir  string
		want bool
	}{
		{"build", true},
		{"build/x", true},
		{"pkg/dist", true},
		{"gen", true},
		{"out/keep", true},
		{"src/a", false},
		{"lib/keep", false},
		{"node_modules/pkg", true},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			w := NewWalker([]string{"node_modules"}, nil)
			w.LoadAncestorRules(filepath.Join(repo, filepath.FromSlash(tt.dir)))
			if w.RootIgnored != tt.want {
				t.Fatalf("RootIgnored = %v, want %v", w.RootIgnored, tt.want)
			}
		})
	}
}

func TestLoadAncestorRules_OutsideRepoUsesWorkingDir(t *testing.T) {
	cwd := t.TempDir()
	if repo, ok := findRepoRoot(cwd); ok {
		t.Skipf("temp dir is inside git repository %s", repo)
	}
	mustWriteFile(t, cwd, ".lxignore", "*.tmp\n")
	mustMkdir(t, cwd, "sub")
	t.Chdir(cwd)

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(filepath.Join(cwd, "sub"))
	if len(w.AncestorRules) != 1 || w.AncestorRules[0].PathOffset != "sub/" {
		t.Fatalf("expected the working directory's rule with offset sub/, got %+v", w.AncestorRules)
	}

	other := t.TempDir()
	w.LoadAncestorRules(other)
	if len(w.AncestorRules) != 0 {
		t.Fatalf("expected no rules for a directory outside the working dir, got %+v", w.AncestorRules)
	}
}
