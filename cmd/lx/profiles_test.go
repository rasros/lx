package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

// The shipped profiles are complete configs of the built-in formats, so
// loading one renders, stats included, exactly as the format's flag does:
// text, binary, unreadable and compact files, sections, prompts and trees.
func TestProfilesMatchBuiltIns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, data := range map[string]string{
		"main.go":    "package main\n\nfunc main() {}\n",
		"notes.txt":  "first\nsecond\n",
		"data.bin":   "\x00\x01\xff\xfe",
		"locked.txt": "secret\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{"main.go", "notes.txt", "data.bin"}
	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		if err := os.Chmod(filepath.Join(dir, "locked.txt"), 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "locked.txt"), 0o644) })
		files = append(files, "locked.txt")
	}
	lookupEnv := hermeticEnv(t, nil)

	run := func(args []string) string {
		var out, errOut bytes.Buffer
		err := cli.Run(context.Background(), append(args, "--stats"), cli.Env{
			Stdin:     strings.NewReader(""),
			Stdout:    &out,
			Stderr:    &errOut,
			LookupEnv: lookupEnv,
			Cwd:       dir,
			TempDir:   t.TempDir(),
		})
		if err != nil {
			t.Fatal(err, errOut.String())
		}
		return out.String() + "\n--- stderr ---\n" + errOut.String()
	}
	runs := [][]string{
		append([]string{"-s", "Code", "-p", "Review this"}, files...),
		append([]string{"-n0"}, files...),
		{"-t", "."},
		{"-s", "One", "main.go", "-s", "Two", "notes.txt"},
	}
	for profile, flag := range map[string]string{"default_config.yaml": "--md", "xml_config.yaml": "--xml"} {
		abs, err := filepath.Abs(filepath.Join("..", "..", profile))
		if err != nil {
			t.Fatal(err)
		}
		for _, args := range runs {
			got := run(append([]string{"-y", abs}, args...))
			want := run(append([]string{flag}, args...))
			if got != want {
				t.Errorf("-y %s %v differs from %s:\n%s\nwant:\n%s", profile, args, flag, got, want)
			}
			if !strings.Contains(want, "main.go") {
				t.Errorf("%s %v rendered no file:\n%s", flag, args, want)
			}
		}
	}
}
