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
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
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
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
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
	dir := t.TempDir()

	makeGitDir(t, dir, "main/.git")
	writeFile(t, dir, "main/.git/info/exclude", "*.secret\n", 0644)
	writeFile(t, dir, "main/.git/worktrees/wt/HEAD", "ref: refs/heads/wt\n", 0644)
	writeFile(t, dir, "main/.git/worktrees/wt/commondir", "../..\n", 0644)
	writeFile(t, dir, "wt/.git", "gitdir: ../main/.git/worktrees/wt\n", 0644)
	writeFile(t, dir, "wt/app.go", "package app", 0644)
	writeFile(t, dir, "wt/key.secret", "excluded by the shared info/exclude", 0644)
	writeFile(t, dir, "wt/pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "wt/pkg/token.secret", "excluded by the shared info/exclude", 0644)

	return filepath.Join(dir, "wt")
}

// SetupStrayGitDirFixture builds a repository holding directories with a
// .git that is not a git directory: an empty one, as a stray mkdir leaves,
// and one with only an info/exclude. Neither is a repository root, so the
// enclosing repository's rules keep applying below them and the stray
// info/exclude is never read. It returns the repository root.
func SetupStrayGitDirFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
	writeFile(t, dir, ".gitignore", "*.log\n", 0644)
	if err := os.MkdirAll(filepath.Join(dir, "empty", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "empty/main.go", "package empty", 0644)
	writeFile(t, dir, "empty/debug.log", "ignored by the repository .gitignore", 0644)
	writeFile(t, dir, "bogus/.git/info/exclude", "*.go\n", 0644)
	writeFile(t, dir, "bogus/util.go", "package bogus", 0644)
	writeFile(t, dir, "bogus/trace.log", "ignored by the repository .gitignore", 0644)

	return dir
}

// SetupStrayGitDirAboveFixture builds a project below a directory holding an
// empty .git and a .gitignore. The empty .git does not make that directory a
// repository, so outside one its .gitignore does not reach a walk started in
// the project. It returns the project directory.
func SetupStrayGitDirAboveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".gitignore", "*.log\n", 0644)
	writeFile(t, dir, "proj/main.go", "package main", 0644)
	writeFile(t, dir, "proj/run.log", "kept: no repository applies the parent .gitignore", 0644)

	return filepath.Join(dir, "proj")
}

// SetupParentIgnoreFilesFixture builds a directory whose .gitignore ignores
// *.tmp and bin/ and whose .ignore ignores *.ign, with a subdirectory src to
// run lx from. With repo set the directory is a git repository root. As in
// ripgrep, .ignore applies either way and .gitignore only inside the
// repository. It returns the directory.
func SetupParentIgnoreFilesFixture(t *testing.T, repo bool) string {
	t.Helper()
	dir := t.TempDir()

	if repo {
		makeGitDir(t, dir, ".git")
	}
	writeFile(t, dir, ".gitignore", "*.tmp\nbin/\n", 0644)
	writeFile(t, dir, ".ignore", "*.ign\n", 0644)
	writeFile(t, dir, "a.tmp", "ignored by .gitignore in a repository", 0644)
	writeFile(t, dir, "bin/x.txt", "ignored by .gitignore in a repository", 0644)
	writeFile(t, dir, "src/ok.txt", "always included", 0644)
	writeFile(t, dir, "src/b.tmp", "ignored by .gitignore in a repository", 0644)
	writeFile(t, dir, "src/c.ign", "always ignored by .ignore", 0644)

	return dir
}

// SetupNegatedIgnoredDirFixture builds a git repository whose .gitignore
// ignores the build directory and tries to re-include one file in it, which
// git and ripgrep do not do, and holds a subdirectory x to run lx from. It
// returns the repository root.
func SetupNegatedIgnoredDirFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
	writeFile(t, dir, ".gitignore", "build/\n!build/keep.js\n", 0644)
	writeFile(t, dir, "build/keep.js", "ignored with its directory despite the negation", 0644)
	writeFile(t, dir, "build/out.js", "ignored with its directory", 0644)
	writeFile(t, dir, "x/main.go", "package x", 0644)

	return dir
}

// SetupIgnoredDirNegationsFixture builds a git repository with negations
// that can and cannot re-include a file: glob/* and deep/** ignore what is
// inside a directory, so a negation re-includes a file there, while nested/
// ignores the directory itself, so its own .gitignore never applies. ovr is
// left to the -e patterns. all, any and two each hold a .gitignore with a
// rule ending in /**.
func SetupIgnoredDirNegationsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	makeGitDir(t, dir, ".git")
	writeFile(t, dir, ".gitignore", "glob/*\n!glob/keep.js\ndeep/**\n!deep/keep.js\nnested/\n", 0644)
	writeFile(t, dir, "glob/keep.js", "re-included: glob/* leaves glob/ itself alone", 0644)
	writeFile(t, dir, "glob/out.js", "ignored by glob/*", 0644)
	writeFile(t, dir, "deep/keep.js", "re-included: deep/** leaves deep/ itself alone", 0644)
	writeFile(t, dir, "deep/out.js", "ignored by deep/**", 0644)
	writeFile(t, dir, "nested/.gitignore", "!keep.js\n", 0644)
	writeFile(t, dir, "nested/keep.js", "ignored with nested/, whose .gitignore is never read", 0644)
	writeFile(t, dir, "ovr/keep.js", "ovr keep", 0644)
	writeFile(t, dir, "ovr/out.js", "ovr out", 0644)
	writeFile(t, dir, "src/main.go", "package src", 0644)
	// A trailing /** needs at least one component below what precedes it.
	for _, p := range []struct{ dir, rule string }{{"all", "**/**"}, {"any", "a/**/**"}, {"two", "a/**/*/**"}} {
		writeFile(t, dir, p.dir+"/.gitignore", p.rule+"\n", 0644)
		writeFile(t, dir, p.dir+"/a/d.txt", "a/d.txt under "+p.rule, 0644)
		writeFile(t, dir, p.dir+"/a/c/e.txt", "a/c/e.txt under "+p.rule, 0644)
		writeFile(t, dir, p.dir+"/z.txt", "z.txt under "+p.rule, 0644)
	}

	return dir
}
