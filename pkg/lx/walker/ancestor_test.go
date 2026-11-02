package walker

import (
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

func TestMatch_PathOffsetNeverExcludesWalkRoot(t *testing.T) {
	rule := offsetRule(t, "dist", "dist/")
	if match(rule, "out.js", false) {
		t.Fatal("expected a rule naming the walk root not to exclude its contents")
	}
	if !match(rule, "nested/dist", true) {
		t.Fatal("expected the rule to still match a matching directory below the root")
	}
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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAncestorRules_StopsAtRepoRoot(t *testing.T) {
	outer := t.TempDir()
	writeTestFile(t, filepath.Join(outer, ".gitignore"), "outer.txt\n")
	repo := filepath.Join(outer, "repo")
	writeTestFile(t, filepath.Join(repo, ".git"), "gitdir: /elsewhere\n")
	writeTestFile(t, filepath.Join(repo, ".gitignore"), "*.log\n")
	writeTestFile(t, filepath.Join(repo, "a", ".ignore"), "tmp/\n")
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(sub)

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

func TestLoadAncestorRules_OutsideRepoUsesWorkingDir(t *testing.T) {
	cwd := t.TempDir()
	writeTestFile(t, filepath.Join(cwd, ".lxignore"), "*.tmp\n")
	sub := filepath.Join(cwd, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(sub)
	if len(w.AncestorRules) != 1 || w.AncestorRules[0].PathOffset != "sub/" {
		t.Fatalf("expected the working directory's rule with offset sub/, got %+v", w.AncestorRules)
	}

	other := t.TempDir()
	w.LoadAncestorRules(other)
	if len(w.AncestorRules) != 0 {
		t.Fatalf("expected no rules for a directory outside the working dir, got %+v", w.AncestorRules)
	}
}
