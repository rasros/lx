//go:build !unix

package main

import "os"

// terminating are the signals that cancel a run.
var terminating = []os.Signal{os.Interrupt}

// dieOf ends a run cancelled by sig with the status a shell reports for an
// interrupt, as there is no signal to re-raise here.
func dieOf(os.Signal) int { return 130 }
