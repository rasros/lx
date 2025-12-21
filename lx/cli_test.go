package lx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// captureStdout temporarily replaces os.Stdout to capture output from Run.
func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	errC := make(chan error, 1)
	go func() {
		errC <- f()
		w.Close()
	}()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	err := <-errC

	os.Stdout = oldStdout
	return buf.String(), err
}

func TestRun_Commands(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantContain []string
	}{
		{
			name:        "help flag",
			args:        []string{"--help"},
			wantContain: []string{"USAGE:", "GLOBAL OPTIONS"},
		},
		{
			name:        "version flag (uppercase V)",
			args:        []string{"-V"},
			wantContain: []string{"lx version"},
		},
		{
			name:        "no args prints help",
			args:        []string{},
			wantContain: []string{"USAGE:"},
		},
		{
			name:        "prompt only",
			args:        []string{"-p", "hello world"},
			wantContain: []string{"hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(func() error {
				return Run(context.Background(), tt.args)
			})
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			for _, s := range tt.wantContain {
				if !strings.Contains(out, s) {
					t.Errorf("Output missing %q. Got:\n%s", s, out)
				}
			}
		})
	}
}

func TestRun_Integration(t *testing.T) {
	// Setup temporary files
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "f1.txt")
	f2 := filepath.Join(tmpDir, "f2.txt")
	longF := filepath.Join(tmpDir, "long.txt")

	// f1: 3 lines
	_ = os.WriteFile(f1, []byte("1-one\n1-two\n1-three\n"), 0644)
	// f2: 4 lines
	_ = os.WriteFile(f2, []byte("2-A\n2-B\n2-C\n2-D\n"), 0644)
	// longF: 10 lines
	var longContent bytes.Buffer
	for i := 1; i <= 10; i++ {
		longContent.WriteString("Line " + strconv.Itoa(i) + "\n")
	}
	_ = os.WriteFile(longF, longContent.Bytes(), 0644)

	tests := []struct {
		name        string
		args        []string
		want        []string // Strings that MUST be present
		wantMissing []string // Strings that MUST NOT be present
	}{
		{
			name: "interleaved head and tail",
			args: []string{"--head", "1", f1, "--tail", "1", f2},
			want: []string{
				"1-one", // f1 head
				"2-D",   // f2 tail
			},
			wantMissing: []string{
				"1-two", // f1 line 2
				"2-A",   // f2 line 1
			},
		},
		{
			name: "sticky flags logic",
			args: []string{"-n2", f2},
			want: []string{
				"2-A", // First line
				"2-D", // Last line
			},
			wantMissing: []string{
				"2-B", "2-C", // Middle lines skipped
			},
		},
		{
			name: "trailing flag logic (lx file -n1)",
			args: []string{f1, "-n1"},
			want: []string{
				"1-one",
			},
			wantMissing: []string{
				"1-two",
				"1-three",
			},
		},
		{
			name: "section header and prompt",
			args: []string{"-s", "MY HEADER", "-p", "MY PROMPT", f1},
			want: []string{
				"## MY HEADER",
				"MY PROMPT",
				"1-one",
			},
		},
		{
			name: "line numbers enabled (-l)",
			args: []string{"-l", f1},
			want: []string{
				"1: 1-one",
			},
		},
		{
			name: "line numbers precedence (explicit -l wins over -L)",
			// Implementation detail: we check -L first, then -l overwrites it.
			// So -L -l results in line numbers.
			args: []string{"-L", "-l", f1},
			want: []string{
				"1: 1-one",
			},
		},
		{
			name: "line numbers precedence (explicit -l wins over -L regardless of order)",
			// Even if -l is first in args, the map doesn't preserve order,
			// but our logic checks L then l, so L is overwritten.
			args: []string{"-l", "-L", f1},
			want: []string{
				"1: 1-one",
			},
		},
		{
			name: "line numbers disabled (-L)",
			args: []string{"-L", f1},
			want: []string{
				"1-one",
			},
			wantMissing: []string{
				"1: 1-one",
			},
		},
		{
			name: "config flag alias (-y)",
			// We can't easily test valid yaml loading here without a file,
			// but we can test that the flag is parsed and passed effectively.
			// For now, just ensure it runs without crashing on argument parsing.
			args: []string{"-y", "missing.yaml", f1},
			// It might fail on file open inside Run, which returns error.
			// Let's expect it to fail:
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(func() error {
				return Run(context.Background(), tt.args)
			})

			// Special handling for the config test which is expected to fail
			if tt.name == "config flag alias (-y)" {
				if err == nil {
					t.Errorf("Expected error for missing config file, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Run() integration error: %v", err)
			}

			for _, s := range tt.want {
				if !strings.Contains(out, s) {
					t.Errorf("Output missing %q.\nOutput:\n%s", s, out)
				}
			}
			for _, s := range tt.wantMissing {
				if strings.Contains(out, s) {
					t.Errorf("Output should NOT contain %q.\nOutput:\n%s", s, out)
				}
			}
		})
	}
}
