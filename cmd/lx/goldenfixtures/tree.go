package goldenfixtures

import (
	"path/filepath"
	"testing"
)

func SetupTreeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "README.md", "# Project", 0644)
	writeFile(t, dir, "src/app.go", "package src", 0644)
	writeFile(t, dir, "src/app_test.go", "package src", 0644)
	writeFile(t, dir, "src/sub/helper.go", "package sub", 0644)
	writeFile(t, dir, "lib/lib.go", "package lib", 0644)
	writeFile(t, dir, "lib/lib_test.go", "package lib", 0644)

	return dir
}

// SetupTreeArchiveFixture is SetupTreeFixture with archive.zip beside its
// files.
func SetupTreeArchiveFixture(t *testing.T) string {
	t.Helper()
	dir := SetupTreeFixture(t)
	createTestZip(filepath.Join(dir, "archive.zip"), [][2]string{
		{"hello.txt", "Hello from archive!\n"},
		{"nested/world.go", "package nested\n"},
		{".hidden_in_zip", "hidden inside zip\n"},
	})
	return dir
}
