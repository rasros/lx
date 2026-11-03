package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/internal/cli"
)

// A run cancelled by a signal that has not ended within this grace period,
// such as one blocked reading a FIFO, is ended by force.
const cancelGrace = 500 * time.Millisecond

// cancelledStatus is the status a shell reports for an interrupted command.
const cancelledStatus = 130

func main() {
	env, err := cli.OSEnv()
	if err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		go func() {
			<-ctx.Done()
			stop() // a second signal kills at once
			time.Sleep(cancelGrace)
			os.Exit(cancelledStatus)
		}()
		err = cli.Run(ctx, os.Args[1:], env)
		if ctx.Err() != nil {
			os.Exit(cancelledStatus)
		}
	}
	if err != nil {
		label := ansi.For(os.Stderr, os.LookupEnv).Paint("Error:", ansi.Bold, ansi.Red)
		fmt.Fprintf(os.Stderr, "%s %v\n", label, err)
		os.Exit(1)
	}
}
