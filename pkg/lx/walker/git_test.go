package walker

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveGitDir returns the git directory of the repository rooted at the
// absolute path repo on the host.
func resolveGitDir(repo string) (string, bool) {
	return findGitDir(hostGitFS{OSHost{}, slog.Default()}, repo)
}

// commonGitDir returns the directory shared by all worktrees of gitDir.
func commonGitDir(gitDir string) string {
	common, _ := commonDir(hostGitFS{OSHost{}, slog.Default()}, gitDir)
	return common
}

// isRepoRoot reports whether dir of a tree is a repository root.
func isRepoRoot(t tree, dir string) bool {
	return newRuleTree(t, true).node(dir).repo
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
			sub := filepath.Join(repo, "sub")
			n := newRuleTree(newHostTree(OSHost{}, sub, slog.Default()), true).node(sub)
			if got := n.parent != nil && n.parent.repo; got != tt.want {
				t.Errorf("rules of sub start at the repository root = %v, want %v", got, tt.want)
			}

			// Within an fs.FS, .git is resolved inside it. An absolute gitdir
			// cannot be followed there but still marks the repository boundary.
			if got := isRepoRoot(newFSTree(os.DirFS(base), slog.Default()), "repo"); got != tt.want {
				t.Errorf("repository root in an fs.FS = %v, want %v", got, tt.want)
			}
			if got := isRepoRoot(newHostTree(OSHost{}, "", slog.Default()), repo); got != tt.want {
				t.Errorf("repository root on the host = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepoRules_UnreachableGitDirKeepsBoundary(t *testing.T) {
	tests := []struct{ name, gitdir string }{
		{"relative gitdir outside the walked fs", "../../R/.git/worktrees/x"},
		{"absolute gitdir", "/nonexistent/R/.git/worktrees/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mustMkdir(t, dir, "wt")
			mustGitDir(t, dir, ".git")
			mustWriteFile(t, dir, ".gitignore", "leak*\n")
			mustWriteFile(t, dir, "wt/.git", "gitdir: "+tt.gitdir+"\n")
			mustWriteFile(t, dir, "wt/leak.txt", "x")

			rt := newRuleTree(newFSTree(os.DirFS(dir), slog.Default()), true)
			if !rt.node("wt").repo {
				t.Fatal("expected a .git file with an unreachable gitdir to stay a repository boundary")
			}
			if rules := rt.frame(newWalker([]string{"*.base"}, nil), "wt").rules; len(rules) != 1 || rules[0].Pattern != "*.base" {
				t.Fatalf("expected only the base rules, got %+v", rules)
			}

			got := walkFrom(t, newWalker(nil, nil), dir, ".")
			if !strings.Contains(strings.Join(got, ","), "wt/leak.txt") {
				t.Fatalf("expected the outer .gitignore to stop at the boundary, walked %v", got)
			}
		})
	}
}

func TestRepoRules_UnreachableCommonDirSkipsInfoExclude(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, dir, "wt/gd/info")
	mustWriteFile(t, dir, "wt/gd/HEAD", "ref: refs/heads/x\n")
	mustWriteFile(t, dir, "wt/gd/commondir", "../../../elsewhere\n")
	mustWriteFile(t, dir, "wt/gd/info/exclude", "*.go\n")
	mustWriteFile(t, dir, "wt/.git", "gitdir: gd\n")

	rt := newRuleTree(newFSTree(os.DirFS(dir), slog.Default()), true)
	ok := rt.node("wt").repo
	if rules := rt.frame(newWalker(nil, nil), "wt").rules; !ok || len(rules) != 0 {
		t.Fatalf("expected a boundary without info/exclude, got %v %+v", ok, rules)
	}
}

func TestValidHead_Symlink(t *testing.T) {
	tests := []struct {
		name, target string
		packed       bool
		want         bool
	}{
		{"unborn branch", "refs/heads/main", false, true},
		{"packed refs", "refs/heads/main", true, true},
		{"outside refs", "../elsewhere/HEAD", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mustMkdir(t, dir, "repo/.git/objects")
			mustMkdir(t, dir, "repo/.git/refs/heads")
			if tt.packed {
				mustWriteFile(t, dir, "repo/.git/packed-refs", strings.Repeat("a", 40)+" refs/heads/main\n")
			}
			if err := os.Symlink(tt.target, filepath.Join(dir, "repo", ".git", "HEAD")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			repo := filepath.Join(dir, "repo")

			if _, got := resolveGitDir(repo); got != tt.want {
				t.Errorf("resolveGitDir = %v, want %v", got, tt.want)
			}
			if got := isRepoRoot(newFSTree(os.DirFS(dir), slog.Default()), "repo"); got != tt.want {
				t.Errorf("repository root in an fs.FS = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidHeadContent(t *testing.T) {
	oid := strings.Repeat("0123456789", 4)
	tests := []struct {
		head string
		want bool
	}{
		{"ref: refs/heads/main", true},
		{"ref: refs/heads/main\n", true},
		{"ref:refs/heads/main\n", true},
		{"ref: heads/main\n", false},
		{oid, true},
		{oid + "\n", true},
		{oid + " extra text\n", true},
		{strings.Repeat("ab", 32) + "\n", true},
		{oid[:39] + "\n", false},
		{"g" + oid[1:] + "\n", false},
		{"", false},
		{"garbage\n", false},
	}
	for _, tt := range tests {
		if got := validHeadContent([]byte(tt.head)); got != tt.want {
			t.Errorf("validHeadContent(%q) = %v, want %v", tt.head, got, tt.want)
		}
	}
}
