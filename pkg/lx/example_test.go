package lx_test

import (
	"context"
	"os"

	"github.com/rasros/lx/pkg/lx"
)

// Example renders a directory as lx <dir> does. The package documentation
// shows this code as its main function, and TestDocExampleCompiles keeps the
// two the same, so the documented usage compiles.
func Example() {
	ctx := context.Background()

	var opts lx.DiscoverOptions
	opts.IgnoreRules, opts.GitExcludes = lx.UserIgnoreRules(os.LookupEnv, nil)
	files, failures, err := lx.NewCollector("", nil).Collect(ctx, ".", opts)
	if err != nil {
		panic(err)
	}

	stream, err := lx.NewStream(lx.NewConfig())
	if err != nil {
		panic(err)
	}
	for _, f := range failures {
		stream.AddFailure(f)
	}
	for _, f := range files {
		stream.AddFile(f, lx.RenderOptions{Head: -1})
	}
	if _, err := stream.Execute(ctx, os.Stdout); err != nil {
		panic(err)
	}
}

// ExampleConfig overrides a layout property of a format's profile: without
// separators, Markdown's files run together.
func ExampleConfig() {
	cfg := lx.NewConfig()
	cfg.Separators = new(false)
	cfg.FileContentTemplate = "{{ .Path }}: {{ .Content }}"

	stream, err := lx.NewStream(cfg)
	if err != nil {
		panic(err)
	}
	stream.AddFile(lx.NewBufferInputFile("a.txt", []byte("one\n")), lx.RenderOptions{Head: -1})
	stream.AddFile(lx.NewBufferInputFile("b.txt", []byte("two\n")), lx.RenderOptions{Head: -1})
	if _, err := stream.Execute(context.Background(), os.Stdout); err != nil {
		panic(err)
	}
	// Output:
	// a.txt: one
	// b.txt: two
}
