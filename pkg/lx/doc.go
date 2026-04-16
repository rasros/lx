// Package lx provides a library for collecting project files and rendering
// them into LLM-friendly output.
//
// Core API:
//
//   - NewConfig creates a Config with sensible defaults.
//   - NewStream creates a Stream that renders files, sections, and prompts.
//   - NewWalker creates a filesystem walker with ignore-rule support.
//   - NewInputFile / NewBufferInputFile create inputs for Stream.AddFile.
//
// Main configuration:
//
//   - Config.OutputFormat controls rendering format: "markdown", "xml", or "html".
//   - Config.ExtractDocuments enables text extraction for .pdf/.docx/.xlsx/.pptx.
//   - Config.IgnoreEnabled and ignore/symlink flags control discovery behavior.
//   - Config template fields (FileContentTemplate, SectionTemplate, etc.) allow
//     custom output structure.
//
// RunnerConfig controls per-file slicing and filtering:
//
//   - Head / Tail choose how many lines to include from start/end.
//   - LineNumbers enables numbered output.
//   - SkeletonFunctions / SkeletonTypes reduce source files to signatures/defs.
//
// Stream methods:
//
//   - AddFile, AddSection, AddPrompt enqueue content.
//   - Execute renders output to an io.Writer.
//   - Prepare / GetGlobalContext expose aggregate statistics.
//   - WithConcurrency / WithTokenizer / WithOnFileError customize processing.
//
// Walker methods:
//
//   - Walk traverses any fs.FS while applying .gitignore, .ignore, and .lxignore
//     semantics (when enabled).
//   - Base and override rules can be passed via NewWalker.
//
// Archive and document helpers:
//
//   - IsArchivePath / ExpandArchive handle archive expansion into stream files.
//   - IsDocumentPath / ExtractDocumentText handle document text extraction.
//
// Typical flow:
//
//  1. Build a Config and Stream.
//  2. Walk a filesystem with Walker, adding files via NewInputFile.
//  3. Render the stream to an io.Writer.
//
// Example:
//
//	package main
//
//	import (
//		"context"
//		"io/fs"
//		"os"
//
//		"github.com/rasros/lx/pkg/lx"
//	)
//
//	func main() {
//		cfg := lx.NewConfig()
//		stream, err := lx.NewStream(cfg, lx.RunnerConfig{Head: -1})
//		if err != nil {
//			panic(err)
//		}
//
//		w := lx.NewWalker(nil, nil)
//		fsys := os.DirFS(".")
//		_ = w.Walk(fsys, ".", func(path string, d fs.DirEntry, err error) error {
//			if err != nil || d.IsDir() {
//				return err
//			}
//			info, err := d.Info()
//			if err != nil {
//				return err
//			}
//			stream.AddFile(lx.NewInputFile(fsys, path, info))
//			return nil
//		})
//
//		_ = stream.Execute(context.Background(), os.Stdout)
//	}
package lx
