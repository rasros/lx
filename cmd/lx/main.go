package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	// The first interrupt or SIGTERM cancels the run, which then cleans up
	// after itself, dropping a partial -o file and stopping profiling. A
	// second one kills the process as usual.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()
	return cli.Run(ctx, os.Args[1:], env)
}
