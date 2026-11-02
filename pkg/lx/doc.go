// Package lx provides a library for collecting project files and rendering
// them into LLM-friendly output, as the lx command does.
//
// Core API:
//
//   - NewCollector finds the inputs a path or URL refers to, as lx does.
//   - NewConfig creates a Config with sensible defaults.
//   - NewStream creates a Stream that renders files, sections, and prompts.
//   - NewInputFile / NewBufferInputFile create inputs of your own.
//
// DiscoverOptions decide what Collector.Collect finds:
//
//   - Includes / Excludes filter the files below a directory or in an archive.
//   - IgnoreRules / GitExcludes apply everywhere; UserIgnoreRules reads the
//     ones lx uses. Ignore files (.gitignore inside a git repository,
//     .ignore and .lxignore everywhere) apply from every directory above the
//     target down, unless NoIgnore is set.
//   - ShowHidden, FollowDirSymlinks and SkipFileSymlinks control the walk.
//   - ExpandArchives collects the files inside .zip, .tar.gz and other
//     archives, local or remote, instead of the archives themselves.
//
// RenderOptions decide how each file added to a Stream renders:
//
//   - Head / Tail choose how many lines to include from start/end; a negative
//     Head renders the whole file, and zero for both only its header.
//   - MaxSize skips files larger than the given byte count; zero disables it.
//   - LineNumbers enables numbered output.
//   - SkeletonFunctions / SkeletonTypes reduce source files to signatures/defs.
//   - ExtractDocuments converts .pdf/.docx/.xlsx/.pptx to text and HTML to
//     Markdown.
//   - ExtractMedia replaces audio and video files with their container metadata.
//
// Main configuration:
//
//   - Config.OutputFormat controls rendering format: "markdown", "xml", "html",
//     or "bare".
//   - Config template fields (FileContentTemplate, SectionTemplate, etc.) allow
//     custom output structure.
//
// Stream methods:
//
//   - AddFile, AddSection, AddPrompt, AddTree, AddMeta enqueue content.
//   - Execute renders output to an io.Writer.
//   - GetGlobalContext exposes aggregate statistics.
//   - WithConcurrency / WithOnFileError customize processing.
//
// Example, rendering a directory as lx <dir> does when no project or user
// lx config changes the output and stats are off:
//
//	package main
//
//	import (
//		"context"
//		"os"
//
//		"github.com/rasros/lx/pkg/lx"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		var opts lx.DiscoverOptions
//		opts.IgnoreRules, opts.GitExcludes = lx.UserIgnoreRules()
//		files, err := lx.NewCollector().Collect(ctx, ".", opts)
//		if err != nil {
//			panic(err)
//		}
//
//		stream, err := lx.NewStream(lx.NewConfig())
//		if err != nil {
//			panic(err)
//		}
//		for _, f := range files {
//			stream.AddFile(f, lx.RenderOptions{Head: -1})
//		}
//		if err := stream.Execute(ctx, os.Stdout); err != nil {
//			panic(err)
//		}
//	}
package lx
