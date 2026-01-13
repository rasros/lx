package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
			wantContain: []string{"SYNOPSIS:", "GLOBAL OPTIONS"},
		},
		{
			name:        "version flag",
			args:        []string{"-V"},
			wantContain: []string{"lx version"},
		},
		{
			name:        "no args implies dot",
			args:        []string{},
			wantContain: []string{"app.go"},
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
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "f1.txt")
	f2 := filepath.Join(tmpDir, "f2.txt")
	longF := filepath.Join(tmpDir, "long.txt")
	emptyF := filepath.Join(tmpDir, "empty.txt")
	outFile := filepath.Join(tmpDir, "out.txt")

	_ = os.WriteFile(f1, []byte("1-one\n1-two\n1-three\n"), 0644)
	_ = os.WriteFile(f2, []byte("2-A\n2-B\n2-C\n2-D\n"), 0644)
	_ = os.WriteFile(emptyF, []byte{}, 0644)

	var longContent bytes.Buffer
	for i := 1; i <= 10; i++ {
		longContent.WriteString("Line " + strconv.Itoa(i) + "\n")
	}
	_ = os.WriteFile(longF, longContent.Bytes(), 0644)

	tests := []struct {
		name        string
		args        []string
		want        []string
		wantMissing []string
	}{
		{
			name:        "interleaved head and tail",
			args:        []string{"--head", "1", f1, "--tail", "1", f2},
			want:        []string{"1-one", "2-D"},
			wantMissing: []string{"1-two", "2-A"},
		},
		{
			name:        "explicit file flag",
			args:        []string{"-f", f1, "-n1"},
			want:        []string{"1-one"},
			wantMissing: []string{"1-two"},
		},
		{
			name: "explicit file flag mixed",
			args: []string{f1, "-f", f2},
			want: []string{"1-one", "2-A"},
		},
		{
			name:        "sticky flags logic",
			args:        []string{"-n2", f2},
			want:        []string{"2-A", "2-D"},
			wantMissing: []string{"2-B", "2-C"},
		},
		{
			name: "trailing prompt",
			args: []string{f1, "-p", "POST_PROMPT"},
			want: []string{"1-one", "POST_PROMPT"},
		},
		{
			name: "section header",
			args: []string{"-s", "MY HEADER", "-p", "MY PROMPT", f1},
			want: []string{"## MY HEADER", "MY PROMPT", "1-one"},
		},
		{
			name: "line numbers enabled",
			args: []string{"-l", f1},
			want: []string{"1: 1-one"},
		},
		{
			name: "line numbers precedence",
			args: []string{"-L", "-l", f1},
			want: []string{"1: 1-one"},
		},
		{
			name:        "line numbers disabled",
			args:        []string{"-L", f1},
			want:        []string{"1-one"},
			wantMissing: []string{"1: 1-one"},
		},
		{
			name: "empty file",
			args: []string{"-f", emptyF},
			want: []string{"empty file\n\n"},
		},
		{
			name: "config flag missing",
			args: []string{"-y", "missing.yaml", f1},
		},
		{
			name:        "missing file check",
			args:        []string{f1, "non_existent_file"},
			want:        []string{"1-one"},
			wantMissing: []string{},
		},
		{
			name:        "compact mode",
			args:        []string{"-n0", f1},
			want:        []string{"(3 rows)"},
			wantMissing: []string{"1-one"},
		},
		{
			name:        "output to file",
			args:        []string{"-o", outFile, f1},
			want:        []string{"Files: 1"},
			wantMissing: []string{"1-one"},
		},
		{
			name:        "quiet output to file",
			args:        []string{"-q", "-o", outFile, f1},
			want:        []string{},
			wantMissing: []string{"Files:"},
		},
		{
			name: "error copy and output",
			args: []string{"-c", "-o", "dummy.txt", f1},
		},
		{
			name: "error copy and stdout",
			args: []string{"-c", "-C", f1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(func() error {
				return Run(context.Background(), tt.args)
			})

			if tt.name == "config flag missing" {
				if err == nil {
					t.Errorf("Expected error for missing config file, got nil")
				}
				return
			}
			if tt.name == "missing file check" {
				if err != nil {
					t.Errorf("Expected nil error for missing file in discovery mode, got %v", err)
				}
				if !strings.Contains(out, "1-one") {
					t.Errorf("Should have printed content for valid file f1")
				}
				return
			}
			if tt.name == "error copy and output" || tt.name == "error copy and stdout" {
				if err == nil {
					t.Errorf("Expected error for conflicting flags, got nil")
				}
				return
			}

			if tt.name == "output to file" || tt.name == "quiet output to file" {
				if err != nil {
					t.Fatalf("Run() error: %v", err)
				}

				for _, s := range tt.want {
					if !strings.Contains(out, s) {
						t.Errorf("Stdout missing %q. Got:\n%s", s, out)
					}
				}
				for _, s := range tt.wantMissing {
					if strings.Contains(out, s) {
						t.Errorf("Stdout should NOT contain %q. Got:\n%s", s, out)
					}
				}

				content, err := os.ReadFile(outFile)
				if err != nil {
					t.Fatalf("Failed to read output file: %v", err)
				}
				if !strings.Contains(string(content), "1-one") {
					t.Errorf("Output file missing content")
				}
				os.Remove(outFile)
				return
			}

			if err != nil {
				t.Fatalf("Run() error: %v", err)
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
