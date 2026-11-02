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
	writeFile(t, dir, "sub/.gitignore", "local.txt\n", 0644)
	writeFile(t, dir, "sub/a.log", "ignored by root pattern", 0644)
	writeFile(t, dir, "sub/keep.log", "re-included by root negation", 0644)
	writeFile(t, dir, "sub/skip.txt", "ignored by anchored root pattern", 0644)
	writeFile(t, dir, "sub/local.txt", "ignored by sub/.gitignore", 0644)
	writeFile(t, dir, "sub/main.go", "package sub", 0644)
	writeFile(t, dir, "sub/deep/b.log", "ignored by root pattern", 0644)
	writeFile(t, dir, "sub/deep/util.go", "package deep", 0644)
	writeFile(t, dir, "dist/out.js", "console.log(1)", 0644)

	return dir
}
