package ansi

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var colorEnv = []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "TERM"}

func clearColorEnv(t *testing.T) {
	t.Helper()
	for _, k := range colorEnv {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestEnabled(t *testing.T) {
	pipe := &bytes.Buffer{}

	tests := []struct {
		name string
		env  map[string]string
		w    io.Writer
		want bool
	}{
		{"unknown writer with no force vars", nil, pipe, false},
		{"NO_COLOR blocks even with FORCE_COLOR", map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1", "TERM": "xterm"}, os.Stdout, false},
		{"CLICOLOR=0 blocks", map[string]string{"CLICOLOR": "0", "FORCE_COLOR": "1", "TERM": "xterm"}, os.Stdout, false},
		{"TERM=dumb blocks", map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, os.Stdout, false},
		{"empty TERM blocks (typical AI agent / CI env)", map[string]string{"FORCE_COLOR": "1"}, os.Stdout, false},
		{"CLICOLOR_FORCE on stdout overrides non-TTY", map[string]string{"CLICOLOR_FORCE": "1", "TERM": "xterm"}, os.Stdout, true},
		{"FORCE_COLOR on stderr overrides non-TTY", map[string]string{"FORCE_COLOR": "1", "TERM": "xterm"}, os.Stderr, true},
		{"plain TTY-less stdout, no force", map[string]string{"TERM": "xterm"}, os.Stdout, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearColorEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := Enabled(tt.w); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPainter(t *testing.T) {
	on, off := New(true), New(false)

	if got := on.Paint("x", Bold, Red); got != "\033[1m\033[31mx\033[0m" {
		t.Errorf("Paint on = %q", got)
	}
	if got := on.Wrap("x", Bold, NoIntensity); got != "\033[1mx\033[22m" {
		t.Errorf("Wrap on = %q", got)
	}
	if got := on.Code(Dim); got != string(Dim) {
		t.Errorf("Code on = %q", got)
	}
	if got := off.Paint("x", Bold, Red) + off.Wrap("y", Bold, NoIntensity) + off.Code(Dim); got != "xy" {
		t.Errorf("disabled painter emitted %q, want plain text", got)
	}
}

// TestNoEscapesOutsidePackage keeps every escape sequence behind a Painter,
// so no output can skip the color check.
func TestNoEscapesOutsidePackage(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	escape := regexp.MustCompile(`\\(033|x1b|x1B|u001b|u001B|e)\[`)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			if path == filepath.Dir(thisFile) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := escape.FindIndex(data); loc != nil {
			line := bytes.Count(data[:loc[0]], []byte("\n")) + 1
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s:%d writes an ANSI escape directly; use internal/ansi", rel, line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
