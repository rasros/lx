package main

import (
	"context"
	"log"
	"os"

	"github.com/rasros/lx/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
