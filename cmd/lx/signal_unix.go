//go:build unix

package main

import (
	"os"
	"syscall"
	"time"
)

// terminating are the signals that cancel a run.
var terminating = []os.Signal{os.Interrupt, syscall.SIGTERM}

// dieOf kills the process with sig, whose handler has been reset, so its
// exit status is the signal's own, 130 for SIGINT and 143 for SIGTERM.
func dieOf(sig os.Signal) int {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return 1
	}
	_ = syscall.Kill(os.Getpid(), s)
	// The signal is delivered asynchronously; should it not end the process,
	// exit with the status a shell would report.
	time.Sleep(time.Second)
	return 128 + int(s)
}
