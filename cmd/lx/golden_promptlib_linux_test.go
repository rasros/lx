package main

import (
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

// A -P that is not a regular file is read in command-line order, so the "-"
// before it reads stdin first and the prompt, stdin again, gets the rest.
func TestGoldenPromptFromStdin(t *testing.T) {
	dir := gf.SetupPromptLibFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "814_promptlib_stdin_read_in_order", args: []string{"-", "-P", "{stdin}"}, stdin: "STDIN\n", stdinPipe: true},
	})
}
