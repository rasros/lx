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
