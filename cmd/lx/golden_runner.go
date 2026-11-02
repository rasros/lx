package main

import (
	"bytes"
	"context"
	"flag"
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
	name     string
	args     []string
	stdin    string
	autoStat bool
}

// goldenRun is what the cases of one batch share: the working directory they
// run in and the paths their output names as /ROOT.
type goldenRun struct {
	cwd   string
	scrub []string
}

// runTestGolden runs cases in workDir, naming it /ROOT.
func runTestGolden(t *testing.T, workDir string, cases []goldenTestCase, extraScrub ...string) {
	t.Helper()
	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)
	runGoldenTests(t, cases, goldenRun{
		cwd:   workDir,
		scrub: append([]string{workDir, canonicalWorkDir}, extraScrub...),
	})
}

// runGoldenTests runs every case with an Env of its own.
func runGoldenTests(t *testing.T, cases []goldenTestCase, run goldenRun) {
	lookupEnv := func(key string) (string, bool) {
		// Goldens are plain text whatever color settings the shell has.
		if key == "NO_COLOR" {
			return "1", true
		}
		return os.LookupEnv(key)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Force deterministic output unless explicitly requested otherwise.
			runArgs := append([]string{}, tc.args...)
			hasStatsControl := false
			for _, a := range runArgs {
				if a == "--stats" || a == "--no-stats" || a == "-q" || a == "--quiet" {
					hasStatsControl = true
				}
			}
			if !hasStatsControl && !tc.autoStat {
				runArgs = append(runArgs, "--no-stats")
			}

			var stdoutBuf, stderrBuf bytes.Buffer
			err := cli.Run(context.Background(), runArgs, cli.Env{
				Stdin:     strings.NewReader(tc.stdin),
				Stdout:    &stdoutBuf,
				Stderr:    &stderrBuf,
				LookupEnv: lookupEnv,
				Cwd:       run.cwd,
				Clipboard: func(string) error {
					t.Error("a golden run wrote to the clipboard")
					return nil
				},
			})
			if err != nil {
				stderrBuf.WriteString("\nCLI Error: " + err.Error() + "\n")
			}

			fullOutput := normalizeOutput(stdoutBuf.String(), stderrBuf.String(), run.scrub...)

			goldenPath := filepath.Join("testdata", "golden", tc.name+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatalf("create golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, []byte(fullOutput), 0644); err != nil {
					t.Fatalf("write golden file: %v", err)
				}
			}

			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				if *update {
					return
				}
				t.Fatalf("Golden file missing: %v. Run make test-update", err)
			}

			want := string(wantBytes)
			if want != fullOutput {
				diffAt := firstDiffIndex(want, fullOutput)
				t.Errorf(
					"Mismatch for %s.\nExpected len: %d\nGot len: %d\nFirst difference at byte: %d\nGolden: testdata/golden/%s.golden\nTo accept current output: make test-update",
					tc.name, len(want), len(fullOutput), diffAt, tc.name,
				)
			}
		})
	}
}

func firstDiffIndex(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}

var (
	tempDirPattern       = regexp.MustCompile(`(/?\w+)+/TestGolden\w+/\d+`)
	permissionPattern    = regexp.MustCompile(`(?i)(permission denied|access is denied)`)
	isDirectoryPattern   = regexp.MustCompile(`(?i)(read .*: is a directory|The handle is invalid)`)
	notFoundPattern      = regexp.MustCompile(`(?i)(The system cannot find the file specified|no such file or directory)`)
	timePattern          = regexp.MustCompile(`time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:[+-]\d{2}:\d{2}|Z)`)
	globalIgnorePattern  = regexp.MustCompile(`msg="Loaded global ignore file" path=.*`)
	systemContextPattern = []struct {
		re   *regexp.Regexp
		repl string
	}{
		// --system-context reports the host machine and the current time.
		{regexp.MustCompile(`(?m)^OS: .*$`), "OS: GOOS"},
		{regexp.MustCompile(`(?m)^Arch: .*$`), "Arch: GOARCH"},
		{regexp.MustCompile(`(?m)^Time: .*$`), "Time: FIXED"},
		{regexp.MustCompile(`(?m)^Host: .*$`), "Host: HOST"},
		{regexp.MustCompile(`(?m)^Tool: lx .*$`), "Tool: lx VERSION"},
	}
)

func normalizeOutput(stdout, stderr string, roots ...string) string {
	var sb strings.Builder

	clean := func(s string) string {
		for _, r := range roots {
			if r != "" {
				s = strings.ReplaceAll(s, r, "/ROOT")
			}
		}

		s = tempDirPattern.ReplaceAllString(s, "/ROOT")

		if runtime.GOOS == "windows" {
			s = strings.ReplaceAll(s, "\\", "/")
		}

		s = permissionPattern.ReplaceAllString(s, "PERMISSION_DENIED")
		s = isDirectoryPattern.ReplaceAllString(s, "IS_DIRECTORY_ERROR")
		s = notFoundPattern.ReplaceAllString(s, "FILE_NOT_FOUND")
		s = timePattern.ReplaceAllString(s, "time=FIXED")
		s = globalIgnorePattern.ReplaceAllString(s, `msg="Loaded global ignore file" path=GLOBAL_IGNORE`)
		for _, p := range systemContextPattern {
			s = p.re.ReplaceAllString(s, p.repl)
		}
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
