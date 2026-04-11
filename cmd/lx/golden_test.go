package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

var update = flag.Bool("update", false, "update .golden files")

type goldenTestCase struct {
	name  string
	args  []string
	stdin string
}

// setupMockConfig installs an empty global lx ignore file so real user config
// does not interfere with tests.
func setupMockConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lx"), 0755)
	os.WriteFile(filepath.Join(dir, "lx", "ignore"), []byte(""), 0644)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func writeFile(t *testing.T, dir, path, content string, perm os.FileMode) {
	t.Helper()
	fp := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fp), err)
	}
	if err := os.WriteFile(fp, []byte(content), perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func makeSymlink(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(filepath.Join(dir, target), fp)
}

func makeSymlinkRaw(dir, target, name string) {
	fp := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(fp), 0755)
	_ = os.Symlink(target, fp)
}

func buildSymlinksDir(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "links/safe_target/recursion.txt", "I am safe", 0644)
	makeSymlink(dir, "main.go", "links/link_to_main.go")
	makeSymlink(dir, "pkg", "links/link_to_pkg")
	makeSymlinkRaw(dir, "does_not_exist", "links/broken_link")
	makeSymlink(dir, "links/safe_target", "links/loop")
	writeFile(t, dir, "links/cycle_a/visible.txt", "a", 0644)
	writeFile(t, dir, "links/cycle_b/visible.txt", "b", 0644)
	makeSymlinkRaw(dir, "../cycle_b", "links/cycle_a/to_b")
	makeSymlinkRaw(dir, "../cycle_a", "links/cycle_b/to_a")
}

func buildLargeFile(t *testing.T, dir, path string) {
	t.Helper()
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	writeFile(t, dir, path, sb.String(), 0644)
}

func runTestGolden(t *testing.T, workDir string, cases []goldenTestCase, extraScrub ...string) {
	t.Helper()
	pkgDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(pkgDir) })
	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	scrub := append([]string{workDir, canonicalWorkDir}, extraScrub...)
	runGoldenTests(t, cases, pkgDir, scrub...)
}

func setupWalkFixture(t *testing.T) string {
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

func setupFormattingFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "README.md", "# Project\nDocumentation here.", 0644)
	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "configs/custom_template.yaml", "file_content_template: \"File: {{ .Path }}\\nContent:\\n{{ .Content }}\"\n", 0644)
	writeFile(t, dir, "configs/custom_sections.yaml", "section_header_template: \"*** {{ .Body }} ***\\n\"\n", 0644)

	return dir
}

func setupSectionsFixture(t *testing.T) string {
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

func setupFilteringFixture(t *testing.T) string {
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

func setupSlicingFixture(t *testing.T) string {
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

func setupSymlinksFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)
	writeFile(t, dir, "pkg/util.go", "package pkg", 0644)
	buildSymlinksDir(t, dir)

	return dir
}

func setupErrorsFixture(t *testing.T) string {
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

func setupStatsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main\nfunc main() {}", 0644)

	return dir
}

func setupDetectionFixture(t *testing.T) string {
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

	return dir
}

func setupConfigFixture(t *testing.T) string {
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

func setupComplexFixture(t *testing.T) string {
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

func setupArchiveFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	createTestZip(filepath.Join(dir, "archive.zip"), [][2]string{
		{"hello.txt", "Hello from archive!\n"},
		{"nested/world.go", "package nested\n"},
		{".hidden_in_zip", "hidden inside zip\n"},
	})
	createTestTarGz(filepath.Join(dir, "archive.tar.gz"), [][2]string{
		{"hello.txt", "Hello from tar!\n"},
		{"nested/world.go", "package nested\n"},
	})
	writeFile(t, dir, "configs/expand.yaml", "expand_archives: true\n", 0644)

	return dir
}

func setupDocumentsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

	fixtures := []string{
		"sample.pdf", "sample.docx", "sample.xlsx",
		"sample.pptx",
		"sample.odt", "sample.ods", "sample.odp",
	}
	for _, name := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", "documents", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	return dir
}

func setupRelativePathsFixture(t *testing.T) (workDir, srcDir string) {
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

func TestGoldenWalk(t *testing.T) {
	dir := setupWalkFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "001_walk_default", args: []string{"."}},
		{name: "002_walk_compact", args: []string{"-n0", "."}},
		{name: "003_specific_file", args: []string{"main.go"}},
		{name: "004_multiple_roots", args: []string{"pkg", "README.md"}},
		{name: "005_walk_subdir", args: []string{"src"}},
		{name: "006_walk_parent_ref", args: []string{"src/../pkg"}},
		{name: "007_walk_dot_slash", args: []string{"./src"}},
	})
}

func TestGoldenFormatting(t *testing.T) {
	dir := setupFormattingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "010_fmt_xml", args: []string{"--xml", "."}},
		{name: "011_fmt_html", args: []string{"--html", "main.go", "README.md"}},
		{name: "012_fmt_markdown_explicit", args: []string{"--md", "main.go"}},
		{name: "013_fmt_xml_sections", args: []string{"--xml", "-s", "Core", "main.go", "-s", "Docs", "README.md"}},
		{name: "014_fmt_html_sections", args: []string{"--html", "-s", "Core", "main.go"}},
		{name: "015_fmt_markdown_no_fences", args: []string{"-y", "configs/custom_template.yaml", "main.go"}},
	})
}

func TestGoldenSections(t *testing.T) {
	dir := setupSectionsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "020_sections_explicit", args: []string{"-s", "Docs", "doc", "-s", "Source", "src"}},
		{name: "021_prompts_mixed", args: []string{"-p", "Analyze this:", "main.go", "-p", "Determine bug"}},
		{name: "022_xml_complex_structure", args: []string{"--xml", "-s", "Context", "-p", "Read carefully", "README.md", "-s", "Code", "main.go"}},
		{name: "023_prompt_only", args: []string{"-p", "Just a question", "-p", "Another question"}},
		{name: "024_section_empty", args: []string{"-s", "Empty Section"}},
		{name: "025_interleaved_prompts_files", args: []string{"main.go", "-p", "Explain above", "pkg/util.go", "-p", "Explain below"}},
	})
}

func TestGoldenFiltering(t *testing.T) {
	dir := setupFilteringFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "030_include_only_go", args: []string{"-i", "*.go", "."}},
		{name: "031_exclude_tests", args: []string{"-e", "*_test.go", "."}},
		{name: "032_mixed_filters", args: []string{"-i", "*.go", "-e", "*_test.go", "."}},
		{name: "033_filter_reset", args: []string{"-i", "*.md", ".", "-E", "-s", "All", "."}},
		{name: "034_hidden_files", args: []string{"-H", "."}},
		{name: "035_no_ignore_files", args: []string{"--no-ignore", "."}},
		{name: "036_exclude_dir", args: []string{"-e", "src", "."}},
		{name: "037_include_path_match", args: []string{"-i", "src/script.py", "."}},
		{name: "038_nested_ignore_file", args: []string{"ignore_test"}},
		{name: "039_complex_globs", args: []string{"-i", "**/*.{go,py}", "-e", "**/main*", "."}},
	})
}

func TestGoldenSlicing(t *testing.T) {
	dir := setupSlicingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "040_lines_limit", args: []string{"--lines", "4", "."}},
		{name: "041_line_numbers", args: []string{"-l", "main.go"}},
		{name: "042_progressive_state", args: []string{
			"-s", "Raw", "README.md",
			"-s", "Numbered", "-l", "main.go",
			"-s", "Sliced", "-n", "2", "pkg/util.go",
			"-s", "Reset", "-L", "-N", "src/script.py",
		}},
		{name: "043_head_tail", args: []string{"--head", "3", "src/large.txt", "--tail", "2", "src/large.txt"}},
		{name: "044_tail_only", args: []string{"--tail", "5", "src/large.txt"}},
		{name: "045_head_only", args: []string{"--head", "5", "src/large.txt"}},
		{name: "046_head_tail_overlap", args: []string{"--head", "100", "--tail", "100", "src/large.txt"}},
		{name: "047_lines_0_compact", args: []string{"-n", "0", "main.go"}},
		{name: "048_slice_binary", args: []string{"--head", "5", "bin/data.bin"}},
	})
}

func TestGoldenSymlinks(t *testing.T) {
	dir := setupSymlinksFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "050_symlinks_default", args: []string{"links"}},
		{name: "051_symlinks_follow", args: []string{"--follow", "links"}},
		{name: "052_symlinks_dag", args: []string{"--follow", "links/loop"}},
		{name: "053_symlinks_infinite_cycle", args: []string{"--follow", "links/cycle_a"}},
		{name: "054_file_links_explicit", args: []string{"--links", "links"}},
		{name: "055_no_file_links", args: []string{"--no-links", "links"}},
		{name: "056_follow_dirs_no_file_links", args: []string{"--follow", "--no-links", "links"}},
		{name: "057_broken_symlink", args: []string{"links/broken_link"}},
	})
}

func TestGoldenErrors(t *testing.T) {
	dir := setupErrorsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "060_access_denied_file", args: []string{"secret/locked.txt"}},
		{name: "061_access_denied_dir", args: []string{"secret/locked_dir"}},
		{name: "062_missing_file", args: []string{"nonexistent.go"}},
		{name: "063_spaces_in_filename", args: []string{"spaces/file with spaces.txt"}},
		{name: "064_spaces_in_dirname", args: []string{"spaces"}},
		{name: "065_stdin_content", args: []string{"-"}, stdin: "Some content from pipe\nLine 2"},
		{name: "066_stdin_and_files", args: []string{"main.go", "-"}, stdin: "Piped content"},
		{name: "067_stdin_file_list", args: []string{}, stdin: "main.go\nREADME.md"},
	})
}

func TestGoldenStats(t *testing.T) {
	dir := setupStatsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "070_stats_forced", args: []string{"--stats", "main.go"}},
		{name: "071_quiet_mode", args: []string{"-q", "main.go"}},
		{name: "072_verbose_debug", args: []string{"-vv", "main.go"}},
		{name: "073_verbose_flag", args: []string{"--verbose=debug", "main.go"}},
		{name: "074_no_stats", args: []string{"--no-stats", "main.go"}},
	})
}

func TestGoldenDetection(t *testing.T) {
	dir := setupDetectionFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "080_binary_detection", args: []string{"bin/data.bin"}},
		{name: "081_empty_file", args: []string{"bin/empty.txt"}},
		{name: "082_lang_detection_rs", args: []string{"langs/main.rs"}},
		{name: "083_lang_detection_py", args: []string{"src/script.py"}},
		{name: "084_lang_detection_dockerfile", args: []string{"langs/Dockerfile"}},
		{name: "085_lang_detection_shebang", args: []string{"langs/script_no_ext"}},
		{name: "086_render_image_md", args: []string{"assets/logo.png"}},
		{name: "087_render_image_html", args: []string{"--html", "assets/logo.png"}},
		{name: "088_render_image_compact", args: []string{"-n0", "assets/logo.png"}},
	})
}

func TestGoldenConfig(t *testing.T) {
	dir := setupConfigFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "090_config_hidden", args: []string{"-y", "configs/hidden.yaml", "."}},
		{name: "091_config_follow", args: []string{"-y", "configs/follow.yaml", "links"}},
		{name: "092_config_override", args: []string{"-y", "configs/follow.yaml", "--no-follow", "links"}},
		{name: "093_force_file_flag", args: []string{"-f", ".gitignore"}},
		{name: "094_force_file_flag_excludes", args: []string{"-e", "*.go", "-f", "main.go"}},
		{name: "095_config_custom_sections", args: []string{"-y", "configs/custom_sections.yaml", "-s", "Head", "main.go"}},
		{name: "096_direct_file_parent_ignore", args: []string{"direct_ignore_test/test.ignored", "direct_ignore_test/test.kept"}},
	})
}

func TestGoldenComplex(t *testing.T) {
	dir := setupComplexFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "100_complex_review_bundle", args: []string{
			"--xml",
			"-p", "Review this code",
			"-s", "Configuration", "-i", "*.yaml", ".",
			"-s", "Source", "-E", "-i", "*.go", "-e", "*_test.go", ".",
			"-s", "Tests", "-E", "-i", "*_test.go", ".",
		}},
		{name: "101_stdin_null_terminated", args: []string{"-0"}, stdin: "main.go\x00README.md\x00spaces/file with spaces.txt\x00"},
		{name: "102_walk_nested_respects_root_ignore", args: []string{"parent_ignore_test/level1/level2"}},
		{name: "103_complex_ignore_exceptions", args: []string{"ignore_exception_test"}},
	})
}

func TestGoldenArchive(t *testing.T) {
	dir := setupArchiveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "110_expand_archive_zip", args: []string{"-Z", "archive.zip"}},
		{name: "111_expand_archive_no_flag", args: []string{"archive.zip"}},
		{name: "112_expand_archive_filter", args: []string{"-Z", "-i", "*.go", "archive.zip"}},
		{name: "113_expand_archive_config", args: []string{"-y", "configs/expand.yaml", "archive.zip"}},
		{name: "114_expand_archive_no_expand_override", args: []string{"-y", "configs/expand.yaml", "--no-expand", "archive.zip"}},
		{name: "115_expand_archive_tar_gz", args: []string{"-Z", "archive.tar.gz"}},
	})
}

func TestGoldenDocuments(t *testing.T) {
	dir := setupDocumentsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "120_docs_pdf_extracted", args: []string{"sample.pdf"}},
		{name: "121_docs_docx_extracted", args: []string{"sample.docx"}},
		{name: "122_docs_xlsx_extracted", args: []string{"sample.xlsx"}},
		{name: "123_docs_no_extract_flag", args: []string{"-D", "sample.pdf", "sample.docx", "sample.xlsx"}},
		{name: "124_docs_odt_expanded", args: []string{"-Z", "sample.odt"}},
		{name: "125_docs_odt_binary", args: []string{"sample.odt"}},
		{name: "126_docs_pptx_extracted", args: []string{"sample.pptx"}},
		{name: "127_docs_ods_expanded", args: []string{"-Z", "sample.ods"}},
		{name: "128_docs_odp_expanded", args: []string{"-Z", "sample.odp"}},
		{name: "129_docs_ods_binary", args: []string{"sample.ods"}},
		{name: "130_docs_odp_binary", args: []string{"sample.odp"}},
	})
}

func TestGoldenRelativePaths(t *testing.T) {
	workDir, srcDir := setupRelativePathsFixture(t)

	pkgDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(pkgDir) })

	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)

	if err := os.Chdir(srcDir); err != nil {
		t.Fatal(err)
	}

	perceivedRoot, _ := filepath.Abs("..")

	cases := []goldenTestCase{
		// --- 200-219: Basic Navigation ---
		{name: "200_relative_parent_file", args: []string{"../README.md"}},
		{name: "201_relative_sibling_file", args: []string{"../pkg/util.go"}},
		{name: "202_relative_sibling_dir", args: []string{"../pkg"}},
		{name: "203_mixed_dots_file", args: []string{"./../README.md"}},
		{name: "204_parent_root_file", args: []string{"../.gitignore"}},
		{name: "205_relative_symlink_target", args: []string{"../links/link_to_main.go"}},
		{name: "206_zig_zag_file", args: []string{"../pkg/../README.md"}},
		{name: "207_current_dir_explicit", args: []string{"./script.py"}},
		{name: "208_redundant_relative", args: []string{"../src/script.py"}},
		{name: "209_relative_binary", args: []string{"../bin/data.bin"}},
		{name: "210_relative_access_denied", args: []string{"../secret/locked.txt"}},
		{name: "211_relative_broken_link", args: []string{"../links/broken_link"}},

		// --- 220-229: Formatting with Relative Paths ---
		{name: "220_relative_xml", args: []string{"--xml", "../README.md"}},
		{name: "221_relative_html_dir", args: []string{"--html", "../pkg"}},
		{name: "222_relative_md_fences", args: []string{"-y", "../configs/custom_template.yaml", "../main.go"}},

		// --- 230-239: Sections & Prompts ---
		{name: "230_section_relative_parent", args: []string{"-s", "Root", "../README.md", "-s", "Lib", "../pkg"}},
		{name: "231_interleaved_relative", args: []string{"../main.go", "-p", "Check this", "../pkg/util.go"}},
		{name: "232_relative_config_loading", args: []string{"-y", "../configs/custom_sections.yaml", "-s", "Parent", "../README.md"}},

		// --- 240-249: Filtering with Relative Roots ---
		{name: "240_relative_include_globs", args: []string{"-i", "*.go", "../pkg"}},
		{name: "241_relative_exclude_parent_dir", args: []string{"-e", "main*", ".."}},
		{name: "242_relative_exclude_nested", args: []string{"-e", "*.txt", "../doc"}},
		{name: "243_relative_hidden_override", args: []string{"-H", "../.hidden"}},
		{name: "244_relative_force_include", args: []string{"-f", "../.gitignore"}},

		// --- 250-259: Slicing with Relative Paths ---
		{name: "250_relative_lines_limit", args: []string{"-n", "2", "../main.go"}},
		{name: "251_relative_head_tail", args: []string{"--head", "2", "--tail", "2", "../src/large.txt"}},
		{name: "252_relative_line_numbers", args: []string{"-l", "../pkg/util.go"}},
		{name: "253_relative_compact", args: []string{"-n", "0", "../README.md"}},

		// --- 260-269: Stdin & Complex ---
		{name: "260_stdin_relative_paths", args: []string{"-"}, stdin: "../README.md\n../pkg/util.go"},
		{name: "261_stdin_relative_null", args: []string{"-0"}, stdin: "../main.go\x00../bin/data.bin\x00"},
		{name: "262_symlink_follow_relative", args: []string{"--follow", "../links/loop"}},
	}

	runGoldenTests(t, cases, pkgDir, workDir, canonicalWorkDir, perceivedRoot)
}

func runGoldenTests(t *testing.T, cases []goldenTestCase, pkgDir string, scrubPaths ...string) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outR, outW, _ := os.Pipe()
			errR, errW, _ := os.Pipe()
			inR, inW, _ := os.Pipe()

			origOut := os.Stdout
			origErr := os.Stderr
			origIn := os.Stdin

			defer func() {
				os.Stdout = origOut
				os.Stderr = origErr
				os.Stdin = origIn
			}()

			os.Stdout = outW
			os.Stderr = errW
			os.Stdin = inR

			go func() {
				defer inW.Close()
				if tc.stdin != "" {
					io.WriteString(inW, tc.stdin)
				}
			}()

			// Ensure stable output for golden files
			runArgs := append([]string{}, tc.args...)
			hasStatsControl := false
			for _, a := range runArgs {
				if a == "--stats" || a == "--no-stats" || a == "-q" || a == "--quiet" {
					hasStatsControl = true
				}
			}
			if !hasStatsControl {
				runArgs = append(runArgs, "--no-stats")
			}

			errChan := make(chan error, 1)
			go func() {
				defer outW.Close()
				defer errW.Close()
				errChan <- cli.Run(context.Background(), runArgs)
			}()

			var stdoutBuf, stderrBuf bytes.Buffer
			_, _ = io.Copy(&stdoutBuf, outR)
			_, _ = io.Copy(&stderrBuf, errR)

			if err := <-errChan; err != nil {
				stderrBuf.WriteString("\nCLI Error: " + err.Error() + "\n")
			}

			fullOutput := normalizeOutput(stdoutBuf.String(), stderrBuf.String(), scrubPaths...)

			goldenPath := filepath.Join(pkgDir, "testdata", "golden", tc.name+".golden")
			if *update {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				os.WriteFile(goldenPath, []byte(fullOutput), 0644)
			}

			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				if *update {
					return
				}
				t.Fatalf("Golden file missing: %v. Run with -update", err)
			}

			if string(wantBytes) != fullOutput {
				t.Errorf("Mismatch for %s.\nExpected len: %d\nGot len: %d\nCheck testdata/golden/%s.golden",
					tc.name, len(wantBytes), len(fullOutput), tc.name)
				_ = os.WriteFile(goldenPath+".actual", []byte(fullOutput), 0644)
			}
		})
	}
}

func normalizeOutput(stdout, stderr string, roots ...string) string {
	var sb strings.Builder

	clean := func(s string) string {
		for _, r := range roots {
			if r != "" {
				s = strings.ReplaceAll(s, r, "/ROOT")
			}
		}

		s = regexp.MustCompile(`(/?\w+)+/TestGolden\w+/\d+`).ReplaceAllString(s, "/ROOT")

		if runtime.GOOS == "windows" {
			s = strings.ReplaceAll(s, "\\", "/")
		}

		s = regexp.MustCompile(`(?i)(permission denied|access is denied)`).ReplaceAllString(s, "PERMISSION_DENIED")
		s = regexp.MustCompile(`(?i)(read .*: is a directory|The handle is invalid)`).ReplaceAllString(s, "IS_DIRECTORY_ERROR")
		s = regexp.MustCompile(`(?i)(The system cannot find the file specified|no such file or directory)`).ReplaceAllString(s, "FILE_NOT_FOUND")
		s = regexp.MustCompile(`time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:[+-]\d{2}:\d{2}|Z)`).ReplaceAllString(s, "time=FIXED")
		s = regexp.MustCompile(`msg="Loaded global ignore file" path=.*`).ReplaceAllString(s, `msg="Loaded global ignore file" path=GLOBAL_IGNORE`)

		return s
	}

	sb.WriteString("--- STDOUT ---\n")
	sb.WriteString(clean(stdout))
	if !strings.HasSuffix(stdout, "\n") {
		sb.WriteString("\n")
	}

	sb.WriteString("\n--- STDERR ---\n")
	stderrClean := clean(stderr)
	lines := strings.Split(strings.TrimSpace(stderrClean), "\n")
	sort.Strings(lines)
	if len(lines) > 0 && lines[0] != "" {
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n")
	}

	return sb.String()
}

func createTestTarGz(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, file := range files {
		body := []byte(file[1])
		hdr := &tar.Header{Name: file[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
		if _, err := tw.Write(body); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	if err := gw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
}

func createTestZip(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestZip: " + err.Error())
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, file := range files {
		fw, err := w.Create(file[0])
		if err != nil {
			panic("createTestZip: " + err.Error())
		}
		if _, err := fw.Write([]byte(file[1])); err != nil {
			panic("createTestZip: " + err.Error())
		}
	}
}
