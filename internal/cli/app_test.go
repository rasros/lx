package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "hello.txt")
	os.WriteFile(f1, []byte("Hello World"), 0644)

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
			args:        []string{"-V"},
			wantContain: []string{"lx version"},
		},
		{
			name:        "render file",
			args:        []string{f1},
			wantContain: []string{"hello.txt", "Hello World"},
		},
		{
			name:        "section and prompt",
			args:        []string{"-s", "MyHeader", "-p", "Instructions", f1},
			wantContain: []string{"## MyHeader", "Instructions", "Hello World"},
		},
		{
			name:        "new flags check",
			args:        []string{"--no-links", "--no-follow", f1},
			wantContain: []string{"hello.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(func() error {
				return Run(context.Background(), tt.args)
			})

			if err != nil && !strings.Contains(tt.name, "error") {
				t.Fatalf("Run() unexpected error: %v", err)
			}

			for _, s := range tt.wantContain {
				if !strings.Contains(out, s) {
					t.Errorf("Output missing %q. Got:\n%s", s, out)
				}
			}
		})
	}
}

func TestRun_StickyFlags(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "long.txt")
	os.WriteFile(f1, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	out, err := captureStdout(func() error {
		return Run(context.Background(), []string{"-n", "2", f1})
	})

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out, "line3") {
		t.Errorf("Output should have been sliced. Got:\n%s", out)
	}
}

func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	errC := make(chan error, 1)
	go func() {
		errC <- f()
		w.Close()
	}()

	var buf strings.Builder
	ioCopyC := make(chan struct{})
	go func() {
		data, _ := io.ReadAll(r)
		buf.Write(data)
		close(ioCopyC)
	}()

	err := <-errC
	<-ioCopyC
	os.Stdout = oldStdout
	return buf.String(), err
}
