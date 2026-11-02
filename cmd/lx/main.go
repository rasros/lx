package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/internal/cli"
)

func main() {
	os.Exit(run())
}

// run runs lx in the process and returns its exit status. Run returns before
// the process exits, so it always stops what it started, such as profiling.
func run() int {
	err := runOS()
	if err == nil {
		return 0
	}
	var exit cli.ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	label := ansi.For(os.Stderr, os.LookupEnv).Paint("Error:", ansi.Bold, ansi.Red)
	fmt.Fprintf(os.Stderr, "%s %v\n", label, err)
	return 1
}

func runOS() error {
	env, err := cli.OSEnv()
	if err != nil {
		return err
	}
	return cli.Run(context.Background(), os.Args[1:], env)
}
