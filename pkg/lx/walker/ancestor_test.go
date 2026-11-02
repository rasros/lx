package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	mustGitDir(t, outer, "gitdirs/repo")
	mustWriteFile(t, repo, ".git", "gitdir: "+filepath.Join(outer, "gitdirs", "repo")+"\n")
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
	mustGitDir(t, repo, ".git")
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

func TestResolveGitDir_Layouts(t *testing.T) {
	base := t.TempDir()

	for _, d := range []string{"plain/.git/info", "plain/.git/worktrees/wt", "plain/.git/modules/mod/info", "plain/mod", "wt"} {
		mustMkdir(t, base, d)
	}
	plain := filepath.Join(base, "plain")
	mustGitDir(t, plain, ".git")
	mustGitDir(t, plain, ".git/modules/mod")
	mustWriteFile(t, plain, ".git/info/exclude", "*.a\n")

	wt := filepath.Join(base, "wt")
	mustWriteFile(t, plain, ".git/worktrees/wt/HEAD", "ref: refs/heads/wt\n")
	mustWriteFile(t, plain, ".git/worktrees/wt/commondir", "../..\n")
	mustWriteFile(t, wt, ".git", "gitdir: ../plain/.git/worktrees/wt\n")

	sub := filepath.Join(plain, "mod")
	mustWriteFile(t, plain, ".git/modules/mod/info/exclude", "*.b\n")
	mustWriteFile(t, sub, ".git", "gitdir: ../.git/modules/mod\n")

	tests := []struct {
		name, repo, want string
	}{
		{"plain repository", plain, filepath.Join(plain, ".git")},
		{"linked worktree shares the main git dir", wt, filepath.Join(plain, ".git")},
		{"submodule uses its own git dir", sub, filepath.Join(plain, ".git", "modules", "mod")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitDir, ok := resolveGitDir(tt.repo)
			if !ok {
				t.Fatal("expected git dir to resolve")
			}
			if got := commonGitDir(gitDir); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadAncestorRules_InfoExcludeComesFirst(t *testing.T) {
	repo := t.TempDir()
	mustGitDir(t, repo, ".git")
	mustMkdir(t, repo, ".git/info")
	mustMkdir(t, repo, "sub")
	mustWriteFile(t, repo, ".git/info/exclude", "*.md\n")
	mustWriteFile(t, repo, ".gitignore", "!README.md\n")
	sub := filepath.Join(repo, "sub")

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(sub)
	if len(w.AncestorRules) != 2 {
		t.Fatalf("expected 2 rules, got %+v", w.AncestorRules)
	}
	if w.AncestorRules[0].Pattern != "*.md" || w.AncestorRules[0].PathOffset != "sub/" {
		t.Fatalf("expected info/exclude rule first with offset sub/, got %+v", w.AncestorRules[0])
	}
	if ignored, _ := checkIgnore("README.md", false, w.AncestorRules, false, false); ignored {
		t.Fatal("expected .gitignore negation to override info/exclude")
	}
	if ignored, _ := checkIgnore("notes.md", false, w.AncestorRules, false, false); !ignored {
		t.Fatal("expected info/exclude to ignore notes.md")
	}

	w.LoadAncestorRules(repo)
	if len(w.AncestorRules) != 0 {
		t.Fatalf("expected no ancestor rules at the repo root, which the walk loads itself, got %+v", w.AncestorRules)
	}
}

func walkAll(t *testing.T, dir string) []string {
	t.Helper()
	w := NewWalker(nil, nil)
	w.FSRoot = dir
	return walkFrom(t, w, dir, ".")
}

func TestWalk_RepoBoundaries(t *testing.T) {
	top := t.TempDir()
	for _, d := range []string{"repo/.git/info", "repo/.git/modules/mod/info", "repo/sub/mod", "repo/sub/inner/.git/info"} {
		mustMkdir(t, top, d)
	}
	for _, d := range []string{"repo/.git", "repo/.git/modules/mod", "repo/sub/inner/.git"} {
		mustGitDir(t, top, d)
	}
	mustWriteFile(t, top, "repo/.git/info/exclude", "*.local\n")
	mustWriteFile(t, top, "repo/.gitignore", "*.tmp\n")
	mustWriteFile(t, top, "repo/a.local", "x")
	mustWriteFile(t, top, "repo/a.go", "x")

	// A submodule: its .git file points into the outer repository.
	mustWriteFile(t, top, "repo/sub/mod/.git", "gitdir: ../../.git/modules/mod\n")
	mustWriteFile(t, top, "repo/.git/modules/mod/info/exclude", "*.b\n")
	mustWriteFile(t, top, "repo/sub/mod/x.b", "x")
	mustWriteFile(t, top, "repo/sub/mod/y.local", "x")
	mustWriteFile(t, top, "repo/sub/mod/z.tmp", "x")

	// A nested repository with its own git directory.
	mustWriteFile(t, top, "repo/sub/inner/.git/info/exclude", "*.c\n")
	mustWriteFile(t, top, "repo/sub/inner/x.c", "x")
	mustWriteFile(t, top, "repo/sub/inner/y.local", "x")

	got := map[string]bool{}
	for _, p := range walkAll(t, top) {
		got[p] = true
	}
	for p, want := range map[string]bool{
		"repo/a.go":              true,
		"repo/a.local":           false, // outer info/exclude, found from above the repo
		"repo/sub/mod/x.b":       false, // the submodule's own info/exclude
		"repo/sub/mod/y.local":   true,  // outer info/exclude stops at the submodule
		"repo/sub/mod/z.tmp":     true,  // outer .gitignore stops at the submodule
		"repo/sub/inner/x.c":     false,
		"repo/sub/inner/y.local": true,
	} {
		if got[p] != want {
			t.Errorf("%s included = %v, want %v (walked %v)", p, got[p], want, got)
		}
	}

	w := NewWalker(nil, nil)
	w.FSRoot = top
	if walked := walkFrom(t, w, top, "repo/a.local"); len(walked) != 0 {
		t.Fatalf("expected a file named from above its repository to honor info/exclude, got %v", walked)
	}
}

func TestResolveRelative_FollowsSymlinkedBase(t *testing.T) {
	top := t.TempDir()
	mustGitDir(t, top, "data/proj/.git/modules/mod")
	mustMkdir(t, top, "data/proj/mod")
	mustMkdir(t, top, "links")
	if err := os.Symlink(filepath.Join(top, "data", "proj", "mod"), filepath.Join(top, "links", "mod")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	mustWriteFile(t, top, "data/proj/mod/.git", "gitdir: ../.git/modules/mod\n")

	gitDir, ok := resolveGitDir(filepath.Join(top, "links", "mod"))
	if !ok {
		t.Fatal("expected git dir to resolve")
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(top, "data", "proj", ".git", "modules", "mod"))
	if gitDir != want {
		t.Fatalf("got %q, want %q", gitDir, want)
	}
}

func TestParseRuleFile_LongLine(t *testing.T) {
	data := "/" + strings.Repeat("a", 70_000) + "\n*.log\n"
	rules := parseRuleFile([]byte(data), ".", "exclude")
	if len(rules) != 2 || rules[1].Pattern != "*.log" {
		t.Fatalf("expected the rule after a long line to survive, got %d rules", len(rules))
	}
}

func TestLoadAncestorRules_InfoExcludeIgnoresParentDir(t *testing.T) {
	repo := t.TempDir()
	mustGitDir(t, repo, ".git")
	mustMkdir(t, repo, ".git/info")
	mustMkdir(t, repo, "scratch/x")
	mustWriteFile(t, repo, ".git/info/exclude", "/scratch/\n")

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(filepath.Join(repo, "scratch", "x"))
	if !w.RootIgnored {
		t.Fatal("expected a directory below an info/exclude entry to be ignored")
	}
}

func TestFindGitDir_Validity(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, base, repo string)
		want  bool
	}{
		{"no .git", func(t *testing.T, base, repo string) {}, false},
		{"empty .git directory", func(t *testing.T, base, repo string) {
			mustMkdir(t, repo, ".git")
		}, false},
		{".git directory with only HEAD", func(t *testing.T, base, repo string) {
			mustMkdir(t, repo, ".git")
			mustWriteFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")
		}, false},
		{".git directory without objects", func(t *testing.T, base, repo string) {
			mustMkdir(t, repo, ".git/refs")
			mustWriteFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")
		}, false},
		{".git directory with an invalid HEAD", func(t *testing.T, base, repo string) {
			mustGitDir(t, repo, ".git")
			mustWriteFile(t, repo, ".git/HEAD", "garbage\n")
		}, false},
		{"valid .git directory", func(t *testing.T, base, repo string) {
			mustGitDir(t, repo, ".git")
		}, true},
		{"valid .git directory with a detached HEAD", func(t *testing.T, base, repo string) {
			mustGitDir(t, repo, ".git")
			mustWriteFile(t, repo, ".git/HEAD", strings.Repeat("ab", 20)+"\n")
		}, true},
		{".git file with a valid relative gitdir", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "store")
			mustWriteFile(t, repo, ".git", "gitdir: ../store\n")
		}, true},
		{".git file with a valid absolute gitdir", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "store")
			mustWriteFile(t, repo, ".git", "gitdir: "+filepath.Join(base, "store")+"\n")
		}, true},
		{".git file with a missing gitdir", func(t *testing.T, base, repo string) {
			mustWriteFile(t, repo, ".git", "gitdir: ../missing\n")
		}, false},
		{".git file pointing at an invalid gitdir", func(t *testing.T, base, repo string) {
			mustMkdir(t, base, "store")
			mustWriteFile(t, repo, ".git", "gitdir: ../store\n")
		}, false},
		{".git file without a gitdir line", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "store")
			mustWriteFile(t, repo, ".git", "../store\n")
		}, false},
		{"empty .git file", func(t *testing.T, base, repo string) {
			mustWriteFile(t, repo, ".git", "")
		}, false},
		{"linked worktree using commondir", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "main/.git")
			mustMkdir(t, base, "main/.git/worktrees/repo")
			mustWriteFile(t, base, "main/.git/worktrees/repo/HEAD", "ref: refs/heads/wt\n")
			mustWriteFile(t, base, "main/.git/worktrees/repo/commondir", "../..\n")
			mustWriteFile(t, repo, ".git", "gitdir: ../main/.git/worktrees/repo\n")
		}, true},
		{"linked worktree whose commondir is not a git dir", func(t *testing.T, base, repo string) {
			mustMkdir(t, base, "main/.git/worktrees/repo")
			mustWriteFile(t, base, "main/.git/worktrees/repo/HEAD", "ref: refs/heads/wt\n")
			mustWriteFile(t, base, "main/.git/worktrees/repo/commondir", "../..\n")
			mustWriteFile(t, repo, ".git", "gitdir: ../main/.git/worktrees/repo\n")
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			mustMkdir(t, base, "repo/sub")
			repo := filepath.Join(base, "repo")
			tt.setup(t, base, repo)

			if _, got := resolveGitDir(repo); got != tt.want {
				t.Errorf("resolveGitDir = %v, want %v", got, tt.want)
			}
			root, found := findRepoRoot(filepath.Join(repo, "sub"))
			if got := found && root == repo; got != tt.want {
				t.Errorf("findRepoRoot found %q (%v), want repo root %v", root, found, tt.want)
			}

			// Without a host path the walk resolves .git within its fs.FS,
			// where an absolute gitdir cannot be followed.
			w := NewWalker(nil, nil)
			_, got := w.repoRules(os.DirFS(base), "repo")
			if want := tt.want && !strings.Contains(tt.name, "absolute"); got != want {
				t.Errorf("repoRules without FSRoot = %v, want %v", got, want)
			}
			w.FSRoot = base
			if _, got := w.repoRules(os.DirFS(base), "repo"); got != tt.want {
				t.Errorf("repoRules with FSRoot = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadAncestorRules_StrayGitDirIsNotARepo(t *testing.T) {
	cwd := t.TempDir()
	mustMkdir(t, cwd, "proj/.git/info")
	mustWriteFile(t, cwd, "proj/.git/info/exclude", "*.go\n")
	mustWriteFile(t, cwd, ".lxignore", "*.tmp\n")
	mustMkdir(t, cwd, "proj/sub")
	if repo, ok := findRepoRoot(cwd); ok {
		t.Skipf("temp dir is inside git repository %s", repo)
	}
	t.Chdir(cwd)

	w := NewWalker(nil, nil)
	w.LoadAncestorRules(filepath.Join(cwd, "proj", "sub"))
	if len(w.AncestorRules) != 1 || w.AncestorRules[0].Pattern != "*.tmp" {
		t.Fatalf("expected only the working directory's rule, not the stray info/exclude, got %+v", w.AncestorRules)
	}
}
