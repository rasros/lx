// Package lx provides a library for collecting project files and rendering
// them into LLM-friendly output, as the lx command does.
//
// Core API:
//
//   - NewCollector finds the inputs a path or URL refers to, as lx does,
//     resolving relative paths against the working directory it is given.
//   - NewConfig creates a Config with sensible defaults.
//   - NewStream creates a Stream that renders files, sections, and prompts.
//   - NewInputFile / NewBufferInputFile create inputs of your own.
//
// DiscoverOptions decide what Collector.Collect finds:
//
//   - Globs filter the files below a directory or in an archive as ripgrep's
//     -g globs do; IncludeGlob and ExcludeGlob make them from lx's -i and -e.
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
//   - AddFile, AddSection, AddPrompt, AddMeta enqueue content.
//   - AddFailure enqueues an input that could not be read, such as one
//     Collect returned, to be reported and counted in output order.
//   - Admit reports whether AddFile keeps a file under its RenderOptions.
//   - AddTree enqueues a directory tree of the files Admit keeps.
//   - Plan reports what the stream would render, as its templates see it.
//   - Execute renders output to an io.Writer once and returns a Report of
//     what it rendered: rows, binary and failed files, bytes and tokens.
//   - WithConcurrency / WithOnFailure / WithLogger customize processing.
//     Skeleton rendering caps the workers at two.
//
// Every input that could not be read is a Failure, which reaches the caller
// once: Collect returns the ones discovery meets, and a Stream hands its
// handler, in output order, the ones added to it and the files it cannot
// read, counting each in the Report's FailedFiles. A Stream with no
// handler logs them.
// The library logs to the *slog.Logger it is given: NewCollector,
// UserIgnoreRules and Stream.WithLogger take one, and nil means
// slog.Default().
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
//		opts.IgnoreRules, opts.GitExcludes = lx.UserIgnoreRules(os.LookupEnv, nil)
//		files, failures, err := lx.NewCollector("", nil).Collect(ctx, ".", opts)
//		if err != nil {
//			panic(err)
//		}
//
//		stream, err := lx.NewStream(lx.NewConfig())
//		if err != nil {
//			panic(err)
//		}
//		for _, f := range failures {
//			stream.AddFailure(f)
//		}
//		for _, f := range files {
//			stream.AddFile(f, lx.RenderOptions{Head: -1})
//		}
//		if _, err := stream.Execute(ctx, os.Stdout); err != nil {
//			panic(err)
//		}
//	}
package lx
