package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
