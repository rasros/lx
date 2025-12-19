package main

import (
	"context"
	"log"
	"os"

	"github.com/rasros/lx/lx"
)

func main() {
	if err := lx.Run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
