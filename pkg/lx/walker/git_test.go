package walker

import (
	"log/slog"
	"path/filepath"
	"testing"
)

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
		{"plain repository", plain, "*.a"},
		{"linked worktree shares the main info/exclude", wt, "*.a"},
		{"submodule uses its own info/exclude", sub, "*.b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := hostTree{log: slog.Default()}
			gitDir, ok := ht.gitDir(tt.repo)
			if !ok {
				t.Fatal("expected a repository root")
			}
			rules := newRuleTree(ht, true).infoExclude(gitDir, ".")
			if len(rules) != 1 || rules[0].Pattern != tt.want {
				t.Fatalf("got %+v, want the rule %q", rules, tt.want)
			}
		})
	}
}
