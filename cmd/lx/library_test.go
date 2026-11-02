package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rasros/lx/internal/cli"
	"github.com/rasros/lx/pkg/lx"
)

// The library flow in the package documentation renders a directory exactly
// as lx <dir> does, ancestor ignore rules included.
func TestLibraryMatchesCLI(t *testing.T) {
	dir := setupAncestorIgnoreFixture(t)
	t.Chdir(filepath.Join(dir, "sub"))
	ctx := context.Background()

	out := filepath.Join(t.TempDir(), "cli.md")
	if err := cli.Run(ctx, []string{".", "--no-stats", "-o", out}); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	var opts lx.DiscoverOptions
	opts.IgnoreRules, opts.GitExcludes = lx.UserIgnoreRules(os.LookupEnv, nil)
	files, err := lx.NewCollector("", nil).Collect(ctx, ".", opts)
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

	if got.String() != string(want) {
		t.Errorf("library output differs from lx .\nlibrary:\n%s\nlx:\n%s", got.String(), want)
	}
	if len(files) < 3 {
		t.Errorf("collected %d files, want the fixture's", len(files))
	}
}
