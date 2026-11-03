package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
)

// The shipped profiles are complete configs of the built-in formats, so
// loading one renders, stats included, exactly as the format's flag does.
func TestProfilesMatchBuiltIns(t *testing.T) {
	t.Parallel()
	dir := setupComplexFixture(t)
	lookupEnv := hermeticEnv(t, nil)
	run := func(args ...string) string {
		var out, errOut bytes.Buffer
		err := cli.Run(context.Background(), append(args, "--stats", "-s", "Code", "-t", "-p", "hi", "."), cli.Env{
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
	for profile, flag := range map[string]string{"default_config.yaml": "--md", "xml_config.yaml": "--xml"} {
		abs, err := filepath.Abs(filepath.Join("..", "..", profile))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := run("-y", abs), run(flag); got != want {
			t.Errorf("-y %s differs from %s:\n%s\nwant:\n%s", profile, flag, got, want)
		}
	}
}
