package goldenfixtures

import (
	"path/filepath"
	"testing"
)

// SetupGlobsFixture builds a repository where -i and -e decide, as
// ripgrep's -g globs do, against a .gitignore, hidden files and an archive:
// build/ holds a file to re-include and a subdirectory, ign/ and app.log are
// ignored by git, and .hdir/ and src/.hidden.go are hidden.
func SetupGlobsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	makeGitDir(t, dir, ".git")

	writeFile(t, dir, ".gitignore", "ign/\n*.log\n", 0644)
	for _, p := range []string{
		"build/keep.js", "build/x.js", "build/sub/y.js",
		"src/a.go", "src/a_test.go", "src/.hidden.go",
		"ign/z.go", "app.log", "ovr/keep.js", "ovr/drop.js",
		".hdir/h.go", "a.go", "b.txt",
	} {
		writeFile(t, dir, p, "x\n", 0644)
	}
	writeFile(t, dir, "arch/.keep", "", 0644)
	createTestZip(filepath.Join(dir, "arch", "a.zip"), [][2]string{
		{"src/a.go", "package src\n"},
		{"notes.txt", "inside the archive\n"},
	})
	return dir
}
