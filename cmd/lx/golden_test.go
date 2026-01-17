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
	// 1. Setup extensive file system fixture
	workDir := setupComplexFixture(t)
	defer os.RemoveAll(workDir)

	// Switch to work dir so "." args work naturally
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
	}{
		// --- Basic Discovery ---
		{name: "01_walk_default", args: []string{"."}},
		{name: "02_walk_compact", args: []string{"-n0", "."}},
		{name: "03_specific_file", args: []string{"main.go"}},
		{name: "04_multiple_roots", args: []string{"pkg", "README.md"}},

		// --- Output Formats ---
		{name: "10_fmt_xml", args: []string{"--xml", "."}},
		{name: "11_fmt_html", args: []string{"--html", "main.go", "README.md"}},
		{name: "12_fmt_markdown_explicit", args: []string{"--md", "main.go"}},

		// --- Structure & Sections ---
		{name: "20_sections_explicit", args: []string{"-s", "Docs", "doc", "-s", "Source", "src"}},
		{name: "21_prompts_mixed", args: []string{"-p", "Analyze this:", "main.go", "-p", "Determine bug"}},
		{name: "22_xml_complex_structure", args: []string{"--xml", "-s", "Context", "-p", "Read carefully", "README.md", "-s", "Code", "main.go"}},

		// --- Filtering ---
		{name: "30_include_only_go", args: []string{"-i", "*.go", "."}},
		{name: "31_exclude_tests", args: []string{"-e", "*_test.go", "."}},
		{name: "32_mixed_filters", args: []string{"-i", "*.go", "-e", "*_test.go", "."}},
		{name: "33_filter_reset", args: []string{"-i", "*.md", ".", "-E", "-s", "All", "."}},
		{name: "34_hidden_files", args: []string{"-H", "."}},
		{name: "35_no_ignore_files", args: []string{"--no-ignore", "."}},

		// --- Interleaved State (The Paintbrush) ---
		{name: "40_lines_limit", args: []string{"--lines", "4", "."}},
		{name: "41_line_numbers", args: []string{"-l", "main.go"}},
		{name: "42_progressive_state", args: []string{
			"-s", "Raw", "README.md",
			"-s", "Numbered", "-l", "main.go",
			"-s", "Sliced", "-n", "2", "pkg/util.go",
			"-s", "Reset", "-L", "-N", "src/script.py",
		}},
		{name: "43_head_tail", args: []string{"--head", "3", "src/large.txt", "--tail", "2", "src/large.txt"}},

		// --- Symlinks & Edge Cases ---
		// 50: Default = Show File Links, Ignore Dir Links
		{name: "50_symlinks_default", args: []string{"links"}},
		// 51: Follow = Show File Links, Recurse Dir Links
		{name: "51_symlinks_follow", args: []string{"--follow", "links"}},
		// 52: DAG Check (A -> B, no cycle)
		{name: "52_symlinks_dag", args: []string{"--follow", "links/loop"}},
		// 53: Cycle Check (A -> B -> A). Should detect and stop.
		{name: "53_symlinks_infinite_cycle", args: []string{"--follow", "links/cycle_a"}},
		// 54: Explicit --links (same as default)
		{name: "54_file_links_explicit", args: []string{"--links", "links"}},
		// 55: --no-links (Hide File Links). Should NOT show link_to_main.go
		{name: "55_no_file_links", args: []string{"--no-links", "links"}},
		// 56: --follow --no-links. Should recurse dirs but hide file links.
		{name: "56_follow_dirs_no_file_links", args: []string{"--follow", "--no-links", "links"}},

		// --- Permissions & Errors ---
		{name: "60_access_denied_file", args: []string{"secret/locked.txt"}},
		{name: "61_access_denied_dir", args: []string{"secret/locked_dir"}},
		{name: "62_missing_file", args: []string{"nonexistent.go"}},

		// --- Stats & Verbosity ---
		{name: "70_stats_forced", args: []string{"--stats", "main.go"}},
		{name: "71_quiet_mode", args: []string{"-q", "main.go"}},
		{name: "72_verbose_debug", args: []string{"-vv", "main.go"}},

		// --- Binary & Empty ---
		{name: "80_binary_detection", args: []string{"bin/data.bin"}},
		{name: "81_empty_file", args: []string{"bin/empty.txt"}},

		// --- Config File Loading ---
		// 90: Config enables hidden files
		{name: "90_config_hidden", args: []string{"-y", "configs/hidden.yaml", "."}},
		// 91: Config enables following symlinks
		{name: "91_config_follow", args: []string{"-y", "configs/follow.yaml", "links"}},
		// 92: CLI overrides Config (Config: follow=true, CLI: --no-follow)
		{name: "92_config_override", args: []string{"-y", "configs/follow.yaml", "--no-follow", "links"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Capture Stdout and Stderr
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

			// 2. Prepare Args
			// We force --no-stats unless the test explicitly requested stats or quiet
			// to keep golden files stable against execution time variations.
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

			// 3. Run Logic
			// We run in a goroutine to ensure pipes don't block if buffer fills
			errChan := make(chan error, 1)
			go func() {
				defer outW.Close()
				defer errW.Close()
				errChan <- cli.Run(context.Background(), runArgs)
			}()

			// 4. Read Output
			var stdoutBuf, stderrBuf bytes.Buffer
			_, _ = io.Copy(&stdoutBuf, outR)
			_, _ = io.Copy(&stderrBuf, errR)

			// Wait for run completion
			if err := <-errChan; err != nil {
				// We don't fail the test on cli.Run error, because some tests
				// intentionally provoke errors (like missing files).
				// We log it to the golden file instead.
				stderrBuf.WriteString("\nCLI Error: " + err.Error() + "\n")
			}

			// 5. Normalize Output (Paths, OS-specific errors)
			fullOutput := normalizeOutput(workDir, stdoutBuf.String(), stderrBuf.String())

			// 6. Compare / Update Golden
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
				// Create a .actual file for debugging
				_ = os.WriteFile(goldenPath+".actual", []byte(fullOutput), 0644)
			}
		})
	}
}

// normalizeOutput combines stdout/stderr and cleans up OS-specific noise
func normalizeOutput(root, stdout, stderr string) string {
	var sb strings.Builder

	clean := func(s string) string {
		// Replace temp dir with constant
		s = strings.ReplaceAll(s, root, "/ROOT")

		// Normalize Windows paths
		if runtime.GOOS == "windows" {
			s = strings.ReplaceAll(s, "\\", "/")
		}

		// Normalize permission errors
		s = regexp.MustCompile(`(?i)(permission denied|access is denied)`).ReplaceAllString(s, "PERMISSION_DENIED")

		// Normalize directory read errors (happens when following directory symlinks)
		// Linux: "read ...: is a directory"
		// Windows: "The handle is invalid" (sometimes) or "is a directory"
		s = regexp.MustCompile(`(?i)(read .*: is a directory|The handle is invalid)`).ReplaceAllString(s, "IS_DIRECTORY_ERROR")

		// Remove timestamp noise from logs
		s = regexp.MustCompile(`time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+[+-]\d{2}:\d{2}`).ReplaceAllString(s, "time=FIXED")

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

	// Helper to create file
	create := func(path, content string, perm os.FileMode) {
		fp := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(fp), 0755)
		if err := os.WriteFile(fp, []byte(content), perm); err != nil {
			t.Fatalf("setup create file %s: %v", path, err)
		}
	}

	// Helper to create symlink (resolves path to absolute)
	symlink := func(oldname, newname string) {
		fp := filepath.Join(dir, newname)
		os.MkdirAll(filepath.Dir(fp), 0755)
		_ = os.Symlink(filepath.Join(dir, oldname), fp)
	}

	// Helper to create RAW symlink (preserves relative paths for cycles)
	symlinkRaw := func(oldname, newname string) {
		fp := filepath.Join(dir, newname)
		os.MkdirAll(filepath.Dir(fp), 0755)
		_ = os.Symlink(oldname, fp)
	}

	// 1. Root Files
	create("README.md", "# Project\nDocumentation here.", 0644)
	create("main.go", "package main\nfunc main() {}", 0644)
	create("main_test.go", "package main\nimport \"testing\"", 0644)
	create(".gitignore", "bin/\nsecret/\n", 0644)
	create(".hidden", "i am hidden", 0644)

	// 2. Subdirectories
	create("pkg/util.go", "package pkg", 0644)
	create("src/script.py", "print('hello')", 0755) // executable
	create("doc/notes.txt", "some notes", 0644)

	// 3. Binary & Empty
	create("bin/empty.txt", "", 0644)
	create("bin/data.bin", string([]byte{0x00, 0x01, 0xFF, 0xFE}), 0644)

	// 4. Large file for slicing
	var large strings.Builder
	for i := 1; i <= 100; i++ {
		large.WriteString("Line ")
		large.WriteString(strings.Repeat("x", 10))
		large.WriteString("\n")
	}
	create("src/large.txt", large.String(), 0644)

	// 5. Symlinks
	symlink("main.go", "links/link_to_main.go")
	symlink("pkg", "links/link_to_pkg")
	symlinkRaw("does_not_exist", filepath.Join(dir, "links/broken_link"))

	create("links/safe_target/recursion.txt", "I am safe", 0644)
	symlink("links/safe_target", "links/loop")

	// TRUE INFINITE CYCLE
	// links/cycle_a/to_b -> ../cycle_b
	// links/cycle_b/to_a -> ../cycle_a
	// FIX: Use visible.txt instead of .keep to verify cycle detection in output
	create("links/cycle_a/visible.txt", "a", 0644)
	create("links/cycle_b/visible.txt", "b", 0644)
	symlinkRaw("../cycle_b", filepath.Join(dir, "links/cycle_a/to_b"))
	symlinkRaw("../cycle_a", filepath.Join(dir, "links/cycle_b/to_a"))

	// 6. Config Files
	create("configs/follow.yaml", "follow_symlinks: true\n", 0644)
	create("configs/hidden.yaml", "show_hidden: true\n", 0644)
	create("configs/no_links.yaml", "no_file_links: true\n", 0644)

	// 7. Permissions (Locked)
	create("secret/locked.txt", "TOP SECRET", 0600) // Start readable to write
	secretDir := filepath.Join(dir, "secret", "locked_dir")
	os.MkdirAll(secretDir, 0755)
	os.WriteFile(filepath.Join(secretDir, "file.txt"), []byte("nested"), 0644)

	// Lock them now
	os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0000)
	os.Chmod(secretDir, 0000)

	t.Cleanup(func() {
		os.Chmod(filepath.Join(dir, "secret/locked.txt"), 0644)
		os.Chmod(secretDir, 0755)
	})

	return dir
}
