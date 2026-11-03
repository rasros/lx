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
		{"valid .git directory", func(t *testing.T, base, repo string) {
			mustGitDir(t, repo, ".git")
		}, true},
		{".git file with a valid relative gitdir", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "store")
			mustWriteFile(t, repo, ".git", "gitdir: ../store\n")
		}, true},
		{".git file with a missing gitdir", func(t *testing.T, base, repo string) {
			mustWriteFile(t, repo, ".git", "gitdir: ../missing\n")
		}, false},
		{"linked worktree using commondir", func(t *testing.T, base, repo string) {
			mustGitDir(t, base, "main/.git")
			mustMkdir(t, base, "main/.git/worktrees/repo")
			mustWriteFile(t, base, "main/.git/worktrees/repo/HEAD", "ref: refs/heads/wt\n")
			mustWriteFile(t, base, "main/.git/worktrees/repo/commondir", "../..\n")
			mustWriteFile(t, repo, ".git", "gitdir: ../main/.git/worktrees/repo\n")
		}, true},
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

func TestValidHeadContent(t *testing.T) {
	oid := strings.Repeat("0123456789", 4)
	tests := []struct {
		head string
		want bool
	}{
		{"ref: refs/heads/main", true},
		{"ref: heads/main\n", false},
		{oid, true},
		{oid[:39] + "\n", false},
		{"garbage\n", false},
	}
	for _, tt := range tests {
		if got := validHeadContent([]byte(tt.head)); got != tt.want {
			t.Errorf("validHeadContent(%q) = %v, want %v", tt.head, got, tt.want)
		}
	}
}
