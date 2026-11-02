package goldenfixtures

import (
	"path/filepath"
	"testing"
)

// SetupIncludePathsFixture builds a tree where -i patterns with a slash
// decide which directories a walk can prune: an archive below arch/, the
// same layout in a plain directory, and directory names that a wildcard or
// an escape in a pattern has to match.
func SetupIncludePathsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "src/b.go", "package src", 0644)
	writeFile(t, dir, "proj/src/c.go", "package src", 0644)
	writeFile(t, dir, "proj/notes.txt", "plain", 0644)
	writeFile(t, dir, "srcfoo/x.go", "package srcfoo", 0644)
	writeFile(t, dir, "lib/x.go", "package lib", 0644)
	writeFile(t, dir, "lit[1]/a.go", "package lit", 0644)
	writeFile(t, dir, "lit1/a.go", "package lit", 0644)
	writeFile(t, dir, "arch/.keep", "", 0644)
	createTestZip(filepath.Join(dir, "arch", "a.zip"), [][2]string{
		{"src/a.go", "package src\n"},
		{"notes.txt", "inside the archive\n"},
	})

	return dir
}
