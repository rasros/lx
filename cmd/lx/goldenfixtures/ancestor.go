package goldenfixtures

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupAncestorIgnoreFixture builds a git repository whose root .gitignore
// must apply when lx walks a subdirectory. It returns the repository root.
func SetupAncestorIgnoreFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".gitignore", "*.log\ndist\n/build\n/sub/skip.txt\n!keep.log\n", 0644)
	writeFile(t, dir, "build/gen.go", "package build", 0644)
	writeFile(t, dir, "sub/a.log", "ignored by root pattern", 0644)
	writeFile(t, dir, "sub/keep.log", "re-included by root negation", 0644)
	writeFile(t, dir, "sub/skip.txt", "ignored by anchored root pattern", 0644)
	writeFile(t, dir, "sub/local.txt", "ignored by sub/.gitignore", 0644)
	writeFile(t, dir, "sub/main.go", "package sub", 0644)
	writeFile(t, dir, "sub/deep/b.log", "ignored by root pattern", 0644)
	writeFile(t, dir, "sub/deep/util.go", "package deep", 0644)
	writeFile(t, dir, "dist/out.js", "console.log(1)", 0644)

	// info/exclude ranks below every .gitignore, so sub/.gitignore can
	// re-include what it excludes.
	writeFile(t, dir, ".git/info/exclude", "# local only\n*.notes\n/sub/scratch/\n*.md\n", 0644)
	writeFile(t, dir, "sub/todo.notes", "excluded by info/exclude", 0644)
	writeFile(t, dir, "sub/scratch/tmp.go", "package scratch", 0644)
	writeFile(t, dir, "sub/README.md", "re-included by sub/.gitignore", 0644)
	writeFile(t, dir, "sub/draft.md", "excluded by info/exclude", 0644)
	writeFile(t, dir, "sub/.gitignore", "local.txt\n!README.md\n", 0644)

	return dir
}

// SetupIgnoredParentDirFixture builds a git repository whose .gitignore
// ignores the build directory with a directory-only rule, which matches no
// file by itself. It returns the repository root.
func SetupIgnoredParentDirFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".gitignore", "build/\n", 0644)
	writeFile(t, dir, "build/out.js", "console.log(1)", 0644)
	writeFile(t, dir, "main.go", "package main", 0644)

	return dir
}

// SetupWorktreeExcludeFixture builds a main repository and a linked worktree
// whose .git file points into it. The worktree shares the main repository's
// info/exclude. It returns the worktree root.
func SetupWorktreeExcludeFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main/.git/info/exclude", "*.secret\n", 0644)
	writeFile(t, dir, "main/.git/worktrees/wt/commondir", "../..\n", 0644)
	writeFile(t, dir, "wt/.git", "gitdir: ../main/.git/worktrees/wt\n", 0644)
	writeFile(t, dir, "wt/app.go", "package app", 0644)
	writeFile(t, dir, "wt/key.secret", "excluded by the shared info/exclude", 0644)
	writeFile(t, dir, "wt/pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "wt/pkg/token.secret", "excluded by the shared info/exclude", 0644)

	return filepath.Join(dir, "wt")
}
