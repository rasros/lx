package main

import "testing"

// A -P that is not a regular file is read in command-line order, so the "-"
// before it reads stdin first and the prompt, stdin again, gets the rest.
func TestGoldenPromptFromStdin(t *testing.T) {
	dir := setupPromptLibFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "814_promptlib_stdin_read_in_order", args: []string{"-", "-P", "{stdin}"}, stdin: "STDIN\n", stdinPipe: true},
	})
}
