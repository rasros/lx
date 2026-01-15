package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

var update = flag.Bool("update", false, "update .golden files")

func TestGolden(t *testing.T) {
	workDir := setupFixture(t)
	defer os.RemoveAll(workDir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{name: "basic_dir_walk", args: []string{"."}},
		{name: "exclude_filter", args: []string{"-e", "*.go", "."}},
		{name: "show_hidden", args: []string{"-H", "."}},
		{name: "ignore_disabled", args: []string{"--no-ignore", "-n0", "."}},
		{name: "compact_view", args: []string{"-n0", "."}},

		{
			name: "md_mixed_prompts_files",
			args: []string{"-p", "Analyze these:", ".", "-p", "End of list"},
		},
		{
			name: "md_explicit_sections",
			args: []string{
				"-s", "Docs", "README.md",
				"-s", "Code", "main.go",
			},
		},

		// --- XML Format Tests ---
		// 1. Implicit Only (Default behavior)
		{
			name: "xml_implicit_basic",
			args: []string{"--xml", "main.go", "README.md"},
		},
		// 2. Implicit with Prompts
		{
			name: "xml_implicit_with_prompts",
			args: []string{"--xml", "-p", "Start", "main.go", "-p", "End"},
		},
		// 3. Explicit Sections Only
		{
			name: "xml_explicit_sections",
			args: []string{"--xml", "-s", "Docs", "README.md", "-s", "Code", "main.go"},
		},
		// 4. Explicit Sections with Prompts inside
		{
			name: "xml_explicit_with_prompts",
			args: []string{
				"--xml",
				"-s", "Context", "-p", "Here is the readme", "README.md",
				"-s", "Task", "-p", "Refactor this", "main.go",
			},
		},
		// 5. Prompts Only (No files)
		{
			name: "xml_prompts_only",
			args: []string{"--xml", "-p", "Just instructions", "-p", "More instructions"},
		},
		// 6. Mixed Implicit then Explicit (Not typical but possible)
		// lx treats the first batch as implicit section 0, then switches to explicit.
		{
			name: "xml_mixed_implicit_explicit",
			args: []string{"--xml", "README.md", "-s", "Code", "main.go"},
		},

		// --- HTML Format Tests ---
		{
			name: "html_basic",
			args: []string{"--html", "main.go"},
		},
		{
			name: "html_explicit_sections",
			args: []string{"--html", "-s", "Documentation", "README.md", "-s", "Code", "main.go"},
		},
		{
			name: "html_prompts_mixed",
			args: []string{"--html", "-p", "Header Prompt", "README.md", "-s", "Code", "-p", "Inner Prompt", "main.go"},
		},

		// --- Complex Filter Logic ---
		{name: "line_numbers", args: []string{"-l", "main.go"}},
		{
			name: "complex_structure",
			args: []string{
				"-s", "Documentation", "-i", "*.md", ".",
				"-E",
				"-s", "Source Code", "-i", "*.go", "-e", "*_test.go", ".",
			},
		},
		{
			name: "filter_reset_relax",
			args: []string{
				"-s", "Go Files Only", "-i", "*.go", ".",
				"-E",
				"-s", "All Files", ".",
			},
		},
		{
			name: "filter_progressive_tightening",
			args: []string{
				"-s", "Round 1 (All)", ".",
				"-e", "*.md",
				"-s", "Round 2 (No Markdown)", ".",
				"-e", "*_test.go",
				"-s", "Round 3 (No MD, No Tests)", ".",
			},
		},
		{
			name: "filter_path_globbing",
			args: []string{
				"-s", "Pkg Directory Only", "-i", "pkg/*", ".",
				"-E",
				"-s", "Root Files Only", "-e", "*/*", ".",
			},
		},
		{
			name: "mixed_sections",
			args: []string{
				"-s", "Docs", "-i", "*.md", ".",
				"-E",
				"-s", "Logic", "-i", "*.go", "-e", "*_test.go", ".",
				"-E",
				"-s", "Scripts", "-i", "myscript", ".",
			},
		},

		// --- Slicing & Edge Cases ---
		{
			name: "progressive_slicing",
			args: []string{
				"-s", "All files compact",
				"-n0", ".",
				"-s", "All go files 1 lines except tests",
				"-n1", "-i", "*.go", "-e", "*_test.go", ".",
				"-NE",
				"-s", "Full main.go",
				"main.go",
			},
		},
		{name: "binary_file", args: []string{"binary.dat"}},
		{name: "shebang_detection", args: []string{"myscript"}},
		{name: "large_file_estimate", args: []string{"--head", "3", "fixtures/large.txt"}},
		{name: "large_file_head", args: []string{"--head", "5", "fixtures/large.txt"}},
		{name: "large_file_gap", args: []string{"--head", "3", "--tail", "3", "fixtures/large.txt"}},
		{name: "large_file_lines_split", args: []string{"--lines", "10", "fixtures/large.txt"}},
		{name: "output_file_flag", args: []string{"-o", "manual_out.txt", "main.go"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outFile := filepath.Join(workDir, "output.tmp")
			runArgs := tc.args

			isManualOut := false
			for _, arg := range runArgs {
				if arg == "-o" || arg == "--output" {
					isManualOut = true
					break
				}
			}

			if !isManualOut {
				runArgs = append(runArgs, "-o", outFile)
			}
			runArgs = append(runArgs, "--no-stats")

			if err := cli.Run(context.Background(), runArgs); err != nil {
				t.Fatalf("cli.Run() failed: %v", err)
			}

			targetFile := outFile
			if isManualOut {
				for i, arg := range tc.args {
					if (arg == "-o" || arg == "--output") && i+1 < len(tc.args) {
						targetFile = filepath.Join(workDir, tc.args[i+1])
					}
				}
			}

			gotBytes, err := os.ReadFile(targetFile)
			if err != nil {
				t.Fatalf("failed to read output: %v", err)
			}
			got := string(gotBytes)

			got = strings.ReplaceAll(got, workDir, "/ROOT")
			if runtime.GOOS == "windows" {
				got = strings.ReplaceAll(got, "\\", "/")
			}

			goldenPath := filepath.Join(wd, "testdata", "golden", tc.name+".golden")
			if *update {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				os.WriteFile(goldenPath, []byte(got), 0644)
			}

			wantBytes, _ := os.ReadFile(goldenPath)
			want := string(wantBytes)

			if got != want {
				t.Errorf("Mismatch for %s. Expected len %d, got %d", tc.name, len(want), len(got))
			}
		})
	}
}

func setupFixture(t *testing.T) string {
	dir := t.TempDir()
	files := map[string]string{
		"README.md":          "# Hello World\nReadme content.",
		"main.go":            "package main\nfunc main() {}",
		"main_test.go":       "package main\nimport \"testing\"",
		"pkg/util/util.go":   "package util",
		"pkg/util/README.md": "# Util Docs",
		"ignore_me.txt":      "secret",
		".gitignore":         "ignore_me.txt\nfixtures/",
		".hidden":            "hidden content",
		"myscript":           "#!/bin/bash\necho 'hi'",
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 'B', 'I', 'N'}
	os.WriteFile(filepath.Join(dir, "binary.dat"), binaryContent, 0644)

	os.MkdirAll(filepath.Join(dir, "fixtures"), 0755)
	var largeBuf bytes.Buffer
	for i := 1; i <= 800; i++ {
		line := fmt.Sprintf("This is deterministic line number %d for estimation testing.\n", i)
		largeBuf.WriteString(line)
	}
	os.WriteFile(filepath.Join(dir, "fixtures", "large.txt"), largeBuf.Bytes(), 0644)

	return dir
}
