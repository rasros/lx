package ansi

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/walker"
)

var colorEnv = []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "TERM"}

// withPipedStdio points stdout and stderr at pipes for the test, so results
// do not depend on whether go test runs in a terminal.
func withPipedStdio(t *testing.T) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	_, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		outW.Close()
		errW.Close()
	})
}

func TestEnabled(t *testing.T) {
	withPipedStdio(t)
	tests := []struct {
		name string
		env  map[string]string
		w    func() io.Writer
		want bool
	}{
		{"unknown writer", map[string]string{"TERM": "xterm"}, func() io.Writer { return &bytes.Buffer{} }, false},
		{"piped stdout", map[string]string{"TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
		{"NO_COLOR beats FORCE_COLOR", map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
		{"CLICOLOR=0 beats FORCE_COLOR", map[string]string{"CLICOLOR": "0", "FORCE_COLOR": "1", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
		{"TERM=dumb beats FORCE_COLOR", map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, func() io.Writer { return os.Stdout }, false},
		{"FORCE_COLOR without TERM", map[string]string{"FORCE_COLOR": "1"}, func() io.Writer { return os.Stdout }, true},
		{"FORCE_COLOR on stderr", map[string]string{"FORCE_COLOR": "1", "TERM": "xterm"}, func() io.Writer { return os.Stderr }, true},
		{"FORCE_COLOR=0 disables", map[string]string{"FORCE_COLOR": "0", "CLICOLOR_FORCE": "1", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
		{"FORCE_COLOR=false disables", map[string]string{"FORCE_COLOR": "false", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
		{"CLICOLOR_FORCE forces", map[string]string{"CLICOLOR_FORCE": "1", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, true},
		{"CLICOLOR_FORCE=0 does not force", map[string]string{"CLICOLOR_FORCE": "0", "TERM": "xterm"}, func() io.Writer { return os.Stdout }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range colorEnv {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := Enabled(tt.w()); got != tt.want {
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
	if got := off.Paint("x", Bold, Red) + off.Wrap("y", Bold, NoIntensity); got != "xy" {
		t.Errorf("disabled painter emitted %q, want plain text", got)
	}
}

var (
	// goEscape matches the spellings of ESC a Go file can use.
	goEscape = regexp.MustCompile(`\\(033|x1[bB]|u001[bB]|U0000001[bB])|rune\(27\)|\x1b`)
	// configEscape matches ESC in YAML and template text, where \e decodes
	// to ESC inside double-quoted YAML strings.
	configEscape = regexp.MustCompile(`\\(e|033|x1[bB]|u001[bB]|U0000001[bB])|\x1b`)
)

// TestNoEscapesOutsidePackage keeps every escape sequence behind a Painter,
// so no output can skip the color check.
func TestNoEscapesOutsidePackage(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(pkgDir, "..", "..")

	// Walk with lx's own walker so gitignored files, such as local checkouts
	// of other repositories, are skipped like hidden and testdata dirs.
	w := walker.New(walker.Options{IgnoreEnabled: true, SkipHidden: true})
	err = w.Walk(os.DirFS(root), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if d.IsDir() {
			if path == pkgDir || d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}

		var pattern *regexp.Regexp
		switch ext := filepath.Ext(path); {
		case ext == ".go" && !strings.HasSuffix(path, "_test.go"):
			pattern = goEscape
		case ext == ".yaml" || ext == ".yml" || ext == ".tmpl":
			pattern = configEscape
		default:
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := pattern.FindIndex(data); loc != nil {
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
