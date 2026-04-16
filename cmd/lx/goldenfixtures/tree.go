package goldenfixtures

import "testing"

func SetupTreeFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
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
