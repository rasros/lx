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
