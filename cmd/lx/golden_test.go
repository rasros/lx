package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

var update = flag.Bool("update", false, "update .golden files")

func TestGolden(t *testing.T) {
	workDir := setupComplexFixture(t)
	defer os.RemoveAll(workDir)

	// Resolve canonical path to handle OS-specific temp directory behaviors (e.g., /tmp vs /private/tmp on macOS)
	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{name: "01_walk_default", args: []string{"."}},
		{name: "02_walk_compact", args: []string{"-n0", "."}},
		{name: "03_specific_file", args: []string{"main.go"}},
		{name: "04_multiple_roots", args: []string{"pkg", "README.md"}},

		{name: "10_fmt_xml", args: []string{"--xml", "."}},
		{name: "11_fmt_html", args: []string{"--html", "main.go", "README.md"}},
		{name: "12_fmt_markdown_explicit", args: []string{"--md", "main.go"}},

		{name: "20_sections_explicit", args: []string{"-s", "Docs", "doc", "-s", "Source", "src"}},
		{name: "21_prompts_mixed", args: []string{"-p", "Analyze this:", "main.go", "-p", "Determine bug"}},
		{name: "22_xml_complex_structure", args: []string{"--xml", "-s", "Context", "-p", "Read carefully", "README.md", "-s", "Code", "main.go"}},

		{name: "30_include_only_go", args: []string{"-i", "*.go", "."}},
		{name: "31_exclude_tests", args: []string{"-e", "*_test.go", "."}},
		{name: "32_mixed_filters", args: []string{"-i", "*.go", "-e", "*_test.go", "."}},
		{name: "33_filter_reset", args: []string{"-i", "*.md", ".", "-E", "-s", "All", "."}},
		{name: "34_hidden_files", args: []string{"-H", "."}},
		{name: "35_no_ignore_files", args: []string{"--no-ignore", "."}},

		{name: "40_lines_limit", args: []string{"--lines", "4", "."}},
		{name: "41_line_numbers", args: []string{"-l", "main.go"}},
		{name: "42_progressive_state", args: []string{
			"-s", "Raw", "README.md",
			"-s", "Numbered", "-l", "main.go",
			"-s", "Sliced", "-n", "2", "pkg/util.go",
			"-s", "Reset", "-L", "-N", "src/script.py",
		}},
		{name: "43_head_tail", args: []string{"--head", "3", "src/large.txt", "--tail", "2", "src/large.txt"}},

		{name: "50_symlinks_default", args: []string{"links"}},
		{name: "51_symlinks_follow", args: []string{"--follow", "links"}},
		{name: "52_symlinks_dag", args: []string{"--follow", "links/loop"}},
		{name: "53_symlinks_infinite_cycle", args: []string{"--follow", "links/cycle_a"}},
		{name: "54_file_links_explicit", args: []string{"--links", "links"}},
		{name: "55_no_file_links", args: []string{"--no-links", "links"}},
		{name: "56_follow_dirs_no_file_links", args: []string{"--follow", "--no-links", "links"}},

		{name: "60_access_denied_file", args: []string{"secret/locked.txt"}},
		{name: "61_access_denied_dir", args: []string{"secret/locked_dir"}},
		{name: "62_missing_file", args: []string{"nonexistent.go"}},

		{name: "70_stats_forced", args: []string{"--stats", "main.go"}},
		{name: "71_quiet_mode", args: []string{"-q", "main.go"}},
		{name: "72_verbose_debug", args: []string{"-vv", "main.go"}},

		{name: "80_binary_detection", args: []string{"bin/data.bin"}},
		{name: "81_empty_file", args: []string{"bin/empty.txt"}},

		{name: "90_config_hidden", args: []string{"-y", "configs/hidden.yaml", "."}},
		{name: "91_config_follow", args: []string{"-y", "configs/follow.yaml", "links"}},
		{name: "92_config_override", args: []string{"-y", "configs/follow.yaml", "--no-follow", "links"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outR, outW, _ := os.Pipe()
			errR, errW, _ := os.Pipe()

			origOut := os.Stdout
			origErr := os.Stderr
			defer func() {
				os.Stdout = origOut
				os.Stderr = origErr
			}()

			os.Stdout = outW
			os.Stderr = errW

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

			fullOutput := normalizeOutput(workDir, canonicalWorkDir, stdoutBuf.String(), stderrBuf.String())

			goldenPath := filepath.Join(wd, "testdata", "golden", tc.name+".golden")
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

func normalizeOutput(root, canonicalRoot, stdout, stderr string) string {
	var sb strings.Builder

	clean := func(s string) string {
		s = strings.ReplaceAll(s, root, "/ROOT")

		if canonicalRoot != "" && canonicalRoot != root {
			s = strings.ReplaceAll(s, canonicalRoot, "/ROOT")
		}

		s = regexp.MustCompile(`(/?\w+)+/TestGolden\d+/\d+`).ReplaceAllString(s, "/ROOT")

		if runtime.GOOS == "windows" {
			s = strings.ReplaceAll(s, "\\", "/")
		}

		s = regexp.MustCompile(`(?i)(permission denied|access is denied)`).ReplaceAllString(s, "PERMISSION_DENIED")
		s = regexp.MustCompile(`(?i)(read .*: is a directory|The handle is invalid)`).ReplaceAllString(s, "IS_DIRECTORY_ERROR")
		s = regexp.MustCompile(`time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:[+-]\d{2}:\d{2}|Z)`).ReplaceAllString(s, "time=FIXED")

		return s
	}

	sb.WriteString("--- STDOUT ---\n")
	sb.WriteString(clean(stdout))
	if !strings.HasSuffix(stdout, "\n") {
		sb.WriteString("\n")
	}

	sb.WriteString("\n--- STDERR ---\n")
	sb.WriteString(clean(stderr))
	if !strings.HasSuffix(stderr, "\n") {
		sb.WriteString("\n")
	}

	return sb.String()
}

func setupComplexFixture(t *testing.T) string {
	dir := t.TempDir()

	create := func(path, content string, perm os.FileMode) {
		fp := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(fp), 0755)
		if err := os.WriteFile(fp, []byte(content), perm); err != nil {
			t.Fatalf("setup create file %s: %v", path, err)
		}
	}

	symlink := func(oldname, newname string) {
		fp := filepath.Join(dir, newname)
		os.MkdirAll(filepath.Dir(fp), 0755)
		_ = os.Symlink(filepath.Join(dir, oldname), fp)
	}

	// symlinkRaw preserves relative paths to allow for testing symlink cycles
	symlinkRaw := func(oldname, newname string) {
		fp := filepath.Join(dir, newname)
		os.MkdirAll(filepath.Dir(fp), 0755)
		_ = os.Symlink(oldname, fp)
	}

	create("README.md", "# Project\nDocumentation here.", 0644)
	create("main.go", "package main\nfunc main() {}", 0644)
	create("main_test.go", "package main\nimport \"testing\"", 0644)
	create(".gitignore", "bin/\nsecret/\n", 0644)
	create(".hidden", "i am hidden", 0644)

	create("pkg/util.go", "package pkg", 0644)
	create("src/script.py", "print('hello')", 0755)
	create("doc/notes.txt", "some notes", 0644)

	create("bin/empty.txt", "", 0644)
	create("bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)

	var large strings.Builder
	for i := 1; i <= 100; i++ {
		large.WriteString("Line ")
		large.WriteString(strings.Repeat("x", 10))
		large.WriteString("\n")
	}
	create("src/large.txt", large.String(), 0644)

	symlink("main.go", "links/link_to_main.go")
	symlink("pkg", "links/link_to_pkg")
	symlinkRaw("does_not_exist", filepath.Join(dir, "links/broken_link"))

	create("links/safe_target/recursion.txt", "I am safe", 0644)
	symlink("links/safe_target", "links/loop")

	create("links/cycle_a/visible.txt", "a", 0644)
	create("links/cycle_b/visible.txt", "b", 0644)
	symlinkRaw("../cycle_b", filepath.Join(dir, "links/cycle_a/to_b"))
	symlinkRaw("../cycle_a", filepath.Join(dir, "links/cycle_b/to_a"))

	create("configs/follow.yaml", "follow_symlinks: true\n", 0644)
	create("configs/hidden.yaml", "show_hidden: true\n", 0644)
	create("configs/no_links.yaml", "no_file_links: true\n", 0644)

	create("secret/locked.txt", "TOP SECRET", 0600)
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
