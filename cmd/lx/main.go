package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/internal/cli"
)

// cleanupGrace is how long a run cancelled by a signal may take to clean up,
// such as dropping its -o buffer and stopping profiling, before the process
// dies of the signal anyway: a run blocked on input that cannot be
// cancelled, such as stdin, must not outlive the signal.
const cleanupGrace = 500 * time.Millisecond

func main() {
	os.Exit(run())
}

// run runs lx in the process and returns its exit status. A terminating
// signal cancels the run and, once it has cleaned up, kills the process as
// the signal would have, so a shell sees the usual status.
func run() int {
	env, err := cli.OSEnv()
	if err != nil {
		return report(err)
	}

	sigs := make(chan os.Signal, 1)
	var caught []os.Signal
	for _, sig := range terminating {
		// A signal the process was started ignoring, as a background job
		// or under nohup is, stays ignored.
		if !signal.Ignored(sig) {
			caught = append(caught, sig)
		}
	}
	if len(caught) > 0 {
		signal.Notify(sigs, caught...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cli.Run(ctx, os.Args[1:], env) }()

	select {
	case err := <-done:
		signal.Stop(sigs)
		return status(err)
	case sig := <-sigs:
		// A second signal kills at once.
		signal.Reset(caught...)
		cancel()
		select {
		case <-done:
		case <-time.After(cleanupGrace):
		}
		return dieOf(sig)
	}
}

// status maps the error Run returned to the exit status, reporting it.
func status(err error) int {
	if err == nil {
		return 0
	}
	var exit cli.ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	return report(err)
}

func report(err error) int {
	label := ansi.For(os.Stderr, os.LookupEnv).Paint("Error:", ansi.Bold, ansi.Red)
	fmt.Fprintf(os.Stderr, "%s %v\n", label, err)
	return 1
}
