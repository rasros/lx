package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/cli"
	"github.com/rasros/lx/pkg/lx"
)

// The library flow in the package documentation renders a directory exactly
// as lx <dir> does, ancestor ignore rules included.
func TestLibraryMatchesCLI(t *testing.T) {
	t.Parallel()
	dir := setupAncestorIgnoreFixture(t)
	cwd := filepath.Join(dir, "sub")
	lookupEnv := hermeticEnv(t, nil)
	ctx := context.Background()

	var want, stderr bytes.Buffer
	err := cli.Run(ctx, []string{".", "--no-stats"}, cli.Env{
		Stdin:     strings.NewReader(""),
		Stdout:    &want,
		Stderr:    &stderr,
		LookupEnv: lookupEnv,
		Cwd:       cwd,
	})
	if err != nil {
		t.Fatal(err, stderr.String())
	}

	var opts lx.DiscoverOptions
	opts.IgnoreRules, opts.GitExcludes = lx.UserIgnoreRules(lookupEnv, nil)
	files, err := lx.NewCollector(cwd, nil).Collect(ctx, ".", opts)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := lx.NewStream(lx.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		stream.AddFile(f, lx.RenderOptions{Head: -1})
	}
	var got bytes.Buffer
	if err := stream.Execute(ctx, &got); err != nil {
		t.Fatal(err)
	}

	if got.String() != want.String() {
		t.Errorf("library output differs from lx .\nlibrary:\n%s\nlx:\n%s", got.String(), want.String())
	}
	if len(files) < 3 {
		t.Errorf("collected %d files, want the fixture's", len(files))
	}
}
