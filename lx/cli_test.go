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
			name:        "version flag",
			args:        []string{"-v"},
			wantContain: []string{"lx version"},
		},
		{
			name:        "no args prints help",
			args:        []string{},
			wantContain: []string{"USAGE:"},
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
			name: "flag state reset check",
			// Ensure setting tail resets head (the bug we fixed)
			args: []string{"--head", "1", f1, "--tail", "1", f1},
			want: []string{
				"1-one",   // First file: Head 1
				"1-three", // Second file arg: Tail 1
			},
		},
		{
			name: "line numbers global",
			// FIXED: Changed -h1 to --head 1 since -h short flag was removed
			args: []string{"-l", "--head", "1", f1},
			want: []string{
				"1: 1-one",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(func() error {
				return Run(context.Background(), tt.args)
			})
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
