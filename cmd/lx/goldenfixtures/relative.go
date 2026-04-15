package goldenfixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func SetupRelativePathsFixture(t *testing.T) (workDir, srcDir string) {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	buildLargeFile(t, dir, "src/large.txt")
	buildSymlinksDir(t, dir)

	writeFile(t, dir, "secret/locked.txt", "TOP SECRET", 0600)
	os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0000)
	t.Cleanup(func() {
		os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0644)
	})

	return dir, filepath.Join(dir, "src")
}
