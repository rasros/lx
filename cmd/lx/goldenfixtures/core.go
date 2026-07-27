package goldenfixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func SetupWalkFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)

	return dir
}

func SetupFormattingFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)

	return dir
}

func SetupSectionsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "doc/notes.txt", "some notes", 0644)

	return dir
}

func SetupFilteringFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "ignore_test/foo.go", "package foo", 0644)
	writeFile(t, dir, "ignore_test/bar.go", "package bar", 0644)
	writeFile(t, dir, "ignore_test/.gitignore", "bar.go", 0644)

	return dir
}

func SetupSlicingFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	buildLargeFile(t, dir, "src/large.txt")

	return dir
}

func SetupSymlinksFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	buildSymlinksDir(t, dir)

	return dir
}

func SetupErrorsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "spaces/file with spaces.txt", "content with spaces", 0644)

	writeFile(t, dir, "secret/locked.txt", "TOP SECRET", 0600)
	secretDir := filepath.Join(dir, "secret", "locked_dir")
	os.MkdirAll(secretDir, 0755)
	os.WriteFile(filepath.Join(secretDir, "file.txt"), []byte("nested"), 0644)
	os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0000)
	os.Chmod(secretDir, 0000)
	t.Cleanup(func() {
		os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0644)
		os.Chmod(secretDir, 0755)
	})

	return dir
}

func SetupStatsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	buildLargeFile(t, dir, "large.txt")

	return dir
}

func SetupDetectionFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)
	writeFile(t, dir, "bin/empty.txt", "", 0644)
	writeFile(t, dir, "langs/main.rs", "fn main() {}", 0644)
	writeFile(t, dir, "langs/Dockerfile", "FROM scratch", 0644)
	writeFile(t, dir, "langs/script_no_ext", "#!/bin/bash\necho hi", 0755)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, "assets/logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\x0D", 0644)
	writeFile(t, dir, "langs/page.html", "<script>alert(1)</script>\n<p>hi &amp; bye</p>", 0644)

	return dir
}

func SetupConfigFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, ".gitignore", "bin/\n*.tmp\n", 0644)
	writeFile(t, dir, ".hidden", "i am hidden", 0644)
	writeFile(t, dir, "configs/follow.yaml", "follow_symlinks: true\n", 0644)
	writeFile(t, dir, "configs/hidden.yaml", "show_hidden: true\n", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	writeFile(t, dir, "direct_ignore_test/.gitignore", "*.ignored\n", 0644)
	writeFile(t, dir, "direct_ignore_test/test.ignored", "should not appear", 0644)
	writeFile(t, dir, "direct_ignore_test/test.kept", "should appear", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/ignore_me.tmp", "ignore", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/keep_me.go", "package level2", 0644)
	buildSymlinksDir(t, dir)

	return dir
}

func SetupComplexFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "main_test.go", "package main\nimport \"testing\"", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	writeFile(t, dir, "src/script.py", "print('hello')", 0755)
	writeFile(t, dir, ".gitignore", "bin/\nsecret/\n*.tmp\n", 0644)
	writeFile(t, dir, "configs/follow.yaml", "follow_symlinks: true\n", 0644)
	writeFile(t, dir, "configs/hidden.yaml", "show_hidden: true\n", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)
	writeFile(t, dir, "spaces/file with spaces.txt", "content with spaces", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/ignore_me.tmp", "ignore", 0644)
	writeFile(t, dir, "parent_ignore_test/level1/level2/keep_me.go", "package level2", 0644)

	ignoreContent := "*\n!/src/**\n!/migrations/**\n!/assets/**\n!/data/**/*.data.xlsx\n!/data/**/index.json\n!langgraph.json\n!pyproject.toml\n!uv.lock\n"
	writeFile(t, dir, "ignore_exception_test/.gitignore", ignoreContent, 0644)
	writeFile(t, dir, "ignore_exception_test/src/main.go", "package main", 0644)
	writeFile(t, dir, "ignore_exception_test/migrations/001_init.sql", "SELECT 1;", 0644)
	writeFile(t, dir, "ignore_exception_test/assets/logo.png", "image_data", 0644)
	writeFile(t, dir, "ignore_exception_test/data/nested/deep/my.data.xlsx", "excel_data", 0644)
	writeFile(t, dir, "ignore_exception_test/data/index.json", "{}", 0644)
	writeFile(t, dir, "ignore_exception_test/langgraph.json", "{}", 0644)
	writeFile(t, dir, "ignore_exception_test/pyproject.toml", "[tool]", 0644)
	writeFile(t, dir, "ignore_exception_test/uv.lock", "lock_data", 0644)
	writeFile(t, dir, "ignore_exception_test/should_ignore.txt", "ignore me", 0644)
	writeFile(t, dir, "ignore_exception_test/data/secret.csv", "1,2,3", 0644)
	writeFile(t, dir, "ignore_exception_test/data/nested/deep/ignore.xlsx", "ignore", 0644)
	writeFile(t, dir, "ignore_exception_test/other_dir/file.go", "package other", 0644)

	return dir
}
