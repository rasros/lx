package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

// usage: go test ./cmd/lx -update
var update = flag.Bool("update", false, "update .golden files")

func TestGolden(t *testing.T) {
	// 1. Setup a temporary workspace with deterministic file content
	workDir := setupFixture(t)
	defer os.RemoveAll(workDir)

	// Ensure we are running inside the temp dir so relative paths in output match
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	// 2. Define test cases
	cases := []struct {
		name string
		args []string
	}{
		// --- BASIC DISCOVERY ---
		{name: "basic_dir_walk", args: []string{"."}},
		{name: "exclude_filter", args: []string{"-e", "*.go", "."}},
		{name: "show_hidden", args: []string{"-H", "."}},
		{name: "ignore_disabled", args: []string{"--no-ignore", "."}},

		// --- FORMATTING MODES ---
		{name: "compact_view", args: []string{"-n0", "."}},
		{name: "xml_format", args: []string{"--xml", "."}},
		{name: "html_format", args: []string{"--html", "."}},
		{name: "line_numbers", args: []string{"-l", "main.go"}},

		// --- COMPLEX COMPOSITION ---
		{
			name: "complex_structure",
			args: []string{
				"-s", "Documentation", "-i", "*.md", ".",
				"-E", // Reset filters
				"-s", "Source Code", "-i", "*.go", "-e", "*_test.go", ".",
			},
		},
		{name: "prompt_injection", args: []string{"-p", "Explain this code", "main.go"}},

		// --- SPECIAL FILE TYPES ---
		{name: "binary_file", args: []string{"binary.dat"}},
		{name: "shebang_detection", args: []string{"myscript"}},

		// --- LARGE FILES & SLICING ---
		{name: "large_file_estimate", args: []string{"large.txt"}},
		{name: "large_file_head", args: []string{"--head", "5", "large.txt"}},
		{name: "large_file_gap", args: []string{"--head", "3", "--tail", "3", "large.txt"}},
		{name: "large_file_lines_split", args: []string{"--lines", "10", "large.txt"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outFile := filepath.Join(workDir, "output.tmp")

			// Force --no-stats to keep stderr clean and output deterministic
			runArgs := append(tc.args, "-o", outFile, "--no-stats")

			if err := cli.Run(context.Background(), runArgs); err != nil {
				t.Fatalf("cli.Run() failed: %v", err)
			}

			gotBytes, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("failed to read output: %v", err)
			}
			got := string(gotBytes)

			// --- SANITIZATION ---
			// 1. Replace random temp dir paths with a constant to satisfy golden comparison
			// This is critical for HTML headers and absolute path references.
			got = strings.ReplaceAll(got, workDir, "/ROOT")

			// 2. Normalize Windows paths for cross-platform consistency
			if runtime.GOOS == "windows" {
				got = strings.ReplaceAll(got, "\\", "/")
			}

			goldenPath := filepath.Join(wd, "testdata", "golden", tc.name+".golden")

			// Update mode: write the current output to the golden file
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			// Comparison mode: read the golden file and compare
			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("Golden file missing: %s. Run 'go test ./cmd/lx -update' to create it.", goldenPath)
				}
				t.Fatal(err)
			}
			want := string(wantBytes)

			if got != want {
				t.Errorf("Output mismatch for %s.\nExpected (len %d):\n%s\nGot (len %d):\n%s",
					tc.name, len(want), want, len(got), got)
			}
		})
	}
}

// setupFixture creates a temporary directory with a known file structure
func setupFixture(t *testing.T) string {
	dir := t.TempDir()

	files := map[string]string{
		"README.md":          "# Hello World\nThis is a readme.",
		"main.go":            "package main\nfunc main() {}",
		"main_test.go":       "package main\nimport \"testing\"",
		"pkg/util/util.go":   "package util",
		"pkg/util/README.md": "# Util Docs",
		"ignore_me.txt":      "secret content",
		".gitignore":         "ignore_me.txt",
		".hidden":            "I am hidden",
		"myscript":           "#!/bin/bash\necho 'hello world'",
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a Binary file (null bytes trigger IsBinary)
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 'B', 'I', 'N', 'A', 'R', 'Y'}
	if err := os.WriteFile(filepath.Join(dir, "binary.dat"), binaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a Large file (>32KB) to trigger line count estimation
	var largeBuf bytes.Buffer
	line := "This is a line used to build a large file for testing token and line estimation logic.\n"
	for i := 0; i < 800; i++ {
		largeBuf.WriteString(line)
	}
	if err := os.WriteFile(filepath.Join(dir, "large.txt"), largeBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}
