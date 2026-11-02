package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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
	// outFile, when set, is a file in the working directory whose content
	// follows the output, or "(missing)".
	outFile string
}

// goldenRun is what the cases of one batch share: the working directory they
// run in, the environment variables set for them and the paths their output
// names as /ROOT.
type goldenRun struct {
	cwd   string
	env   map[string]string
	scrub []string
}

// runTestGolden runs cases in workDir, naming it /ROOT.
func runTestGolden(t *testing.T, workDir string, cases []goldenTestCase, extraScrub ...string) {
	t.Helper()
	runTestGoldenEnv(t, workDir, nil, cases, extraScrub...)
}

// runTestGoldenEnv runs cases in workDir, as runTestGolden, with the
// environment variables env set.
func runTestGoldenEnv(t *testing.T, workDir string, env map[string]string, cases []goldenTestCase, extraScrub ...string) {
	t.Helper()
	canonicalWorkDir, _ := filepath.EvalSymlinks(workDir)
	runGoldenTests(t, cases, goldenRun{
		cwd:   workDir,
		env:   env,
		scrub: append([]string{workDir, canonicalWorkDir}, extraScrub...),
	})
}

// runGoldenTests runs every case in parallel, each with an Env of its own, so
// nothing a case sees of the process can leak between them. The test and its
// cases share their files, so a batch must not change them.
func runGoldenTests(t *testing.T, cases []goldenTestCase, run goldenRun) {
	t.Parallel()
	vars := map[string]string{
		// Goldens are plain text whatever color settings the shell has.
		"NO_COLOR": "1",
		// The user's own config never applies; lx/ignore is empty.
		"XDG_CONFIG_HOME": mockConfigDir(t),
		"LX_CONFIG":       "",
		"LX_PROMPTS_DIR":  "",
	}
	for k, v := range run.env {
		vars[k] = v
	}
	lookupEnv := func(key string) (string, bool) {
		if v, ok := vars[key]; ok {
			return v, v != ""
		}
		return os.LookupEnv(key)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
			if tc.outFile != "" {
				content := "(missing)\n"
				if data, err := os.ReadFile(filepath.Join(run.cwd, tc.outFile)); err == nil {
					content = string(data)
				}
				fullOutput += "\n--- " + tc.outFile + " ---\n" + content
			}

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

// mockConfigDir returns a user config directory whose lx/ignore is empty.
func mockConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lx"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lx", "ignore"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
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

	// Stderr is kept in the order it was written, which is part of the
	// behavior: render workers log nothing but hand what went wrong back to
	// the goroutine writing the output, so the order is deterministic.
	sb.WriteString("\n--- STDERR ---\n")
	if s := strings.TrimSpace(clean(stderr)); s != "" {
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	return sb.String()
}
