package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
		label := ansi.For(os.Stderr).Paint("Error:", ansi.Bold, ansi.Red)
		fmt.Fprintf(os.Stderr, "%s %v\n", label, err)
		os.Exit(1)
	}
}
