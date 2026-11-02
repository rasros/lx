package goldenfixtures

import (
	"testing"
)

// SetupNestedReposInIgnoredDirsFixture builds a git repository that ignores
// build/ and .worktrees/, holding a nested repository in build/inner and a
// linked worktree of itself in .worktrees/wt. Inside either, its own rules
// decide. It returns the outer repository root.
func SetupNestedReposInIgnoredDirsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
	writeFile(t, dir, ".gitignore", "build/\n.worktrees/\n", 0644)
	writeFile(t, dir, "main.go", "package main", 0644)
	writeFile(t, dir, "build/out.js", "ignored with build/", 0644)

	makeGitDir(t, dir, "build/inner/.git")
	writeFile(t, dir, "build/inner/.gitignore", "*.log\n", 0644)
	writeFile(t, dir, "build/inner/i.go", "package inner", 0644)
	writeFile(t, dir, "build/inner/i.log", "ignored by the nested repository", 0644)

	writeFile(t, dir, ".git/worktrees/wt/HEAD", "ref: refs/heads/wt\n", 0644)
	writeFile(t, dir, ".git/worktrees/wt/commondir", "../..\n", 0644)
	writeFile(t, dir, ".worktrees/wt/.git", "gitdir: ../../.git/worktrees/wt\n", 0644)
	writeFile(t, dir, ".worktrees/wt/.gitignore", "build/\n.worktrees/\n", 0644)
	writeFile(t, dir, ".worktrees/wt/main.go", "package main // worktree", 0644)
	writeFile(t, dir, ".worktrees/wt/build/out.js", "ignored by the worktree's .gitignore", 0644)

	return dir
}

// SetupSymlinkedRootsFixture builds a repository ignoring *.log and a
// directory outside it, with srclink pointing at the repository's src and
// repo/lnk pointing at the outside directory. As in ripgrep, a walked root
// takes the rules of the directories above its target. It returns the
// directory holding them.
func SetupSymlinkedRootsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	makeGitDir(t, dir, "repo/.git")
	writeFile(t, dir, "repo/.gitignore", "*.log\n", 0644)
	writeFile(t, dir, "repo/src/a.go", "package src", 0644)
	writeFile(t, dir, "repo/src/b.log", "ignored by the repository", 0644)
	writeFile(t, dir, "other/o.go", "package other", 0644)
	writeFile(t, dir, "other/o.log", "outside the repository", 0644)
	makeSymlinkRaw(dir, "repo/src", "srclink")
	makeSymlinkRaw(dir, "../other", "repo/lnk")

	return dir
}

// SetupIgnoreSyntaxFixture builds a repository whose .gitignore re-includes
// hidden paths with negations, which wins over skipping hidden files as in
// ripgrep, ends patterns with spaces, which git strips unless escaped, and
// escapes a star and a space with backslashes.
// It returns the repository root.
func SetupIgnoreSyntaxFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
	writeFile(t, dir, ".gitignore", "!.kunitconfig\n!.github/\ntrail\\ \ntrail2   \n\\*.go\na\\ b.go\n", 0644)
	writeFile(t, dir, "*.go", "ignored: the escaped star is literal", 0644)
	writeFile(t, dir, "x.go", "kept: \\*.go names only *.go", 0644)
	writeFile(t, dir, "a b.go", "ignored by the escaped space", 0644)
	writeFile(t, dir, ".kunitconfig", "re-included by a negation", 0644)
	writeFile(t, dir, ".hidden", "hidden", 0644)
	writeFile(t, dir, ".github/workflow.yml", "on: push", 0644)
	writeFile(t, dir, ".other/x.txt", "hidden directory", 0644)
	writeFile(t, dir, "trail ", "ignored by the escaped trailing space", 0644)
	writeFile(t, dir, "trail2", "ignored, the trailing spaces stripped", 0644)
	writeFile(t, dir, "trail", "kept: the pattern ends in a space", 0644)

	return dir
}
