// Package lx provides a library for discovering, slicing, and formatting file system content
// specifically for Large Language Model (LLM) contexts.
//
// It replicates the core logic of the 'lx' CLI tool, allowing developers to embed
// file walking, gitignore processing, token estimation, and Markdown/XML rendering
// into their own applications.
//
// The core workflow involves two main components:
//
// 1. Walker: Recursively discovers files while respecting .gitignore, .ignore, and .lxignore files.
//
// 2. Stream: Manages the collection of files, prompts, and sections, and renders them
// into a single coherent output.
//
// Example: Processing a ZIP archive (Virtual FS)
//
//	package main
//
//	import (
//	    "archive/zip"
//	    "context"
//	    "os"
//
//	    "github.com/rasros/lx/pkg/lx"
//	)
//
//	func main() {
//	    // 1. Open a zip file (works with any fs.FS)
//	    r, _ := zip.OpenReader("repo.zip")
//	    defer r.Close()
//
//	    // 2. Configure the generic walker
//	    walker := lx.NewWalker(lx.WalkerOptions{
//	        FS:            r,
//	        IgnoreEnabled: true, // respects .gitignore inside the zip
//	    })
//
//	    // 3. Setup the output stream (Markdown format)
//	    config := lx.NewConfig()
//	    stream, _ := lx.NewStream(config, lx.RunnerConfig{})
//
//	    // 4. Walk and stream
//	    ctx := context.Background()
//	    for file := range walker.Walk(ctx, []string{"."}) {
//	        stream.AddFile(file)
//	    }
//
//	    // 5. Render to stdout
//	    stream.Execute(ctx, os.Stdout)
//	}
package lx
