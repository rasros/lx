// Package lx provides a library for collecting project files and rendering
// them into LLM-friendly output, as the lx command does.
//
// Core API:
//
//   - [NewCollector] finds the inputs a path or URL refers to, as lx does,
//     resolving relative paths against the working directory it is given.
//   - [NewConfig] creates a [Config] with sensible defaults, and [Merge]
//     layers one Config over another.
//   - [NewStream] creates a [Stream] that renders files, sections, and
//     prompts.
//   - [NewInputFile] / [NewBufferInputFile] create inputs of your own.
//
// [DiscoverOptions] decide what [Collector.Collect] finds:
//
//   - [DiscoverOptions.Globs] filter the files below a directory or in an
//     archive as ripgrep's -g globs do; [IncludeGlob] and [ExcludeGlob] make
//     them from lx's -i and -e.
//   - [DiscoverOptions.IgnoreRules] / [DiscoverOptions.GitExcludes] apply
//     everywhere; [UserIgnoreRules] reads the ones lx uses. Ignore files
//     (.gitignore inside a git repository, .ignore and .lxignore everywhere)
//     apply from every directory above the target down, unless
//     [DiscoverOptions.NoIgnore] is set.
//   - [DiscoverOptions.ShowHidden], [DiscoverOptions.FollowDirSymlinks] and
//     [DiscoverOptions.SkipFileSymlinks] control the walk.
//   - [DiscoverOptions.ExpandArchives] collects the files inside .zip,
//     .tar.gz and other archives, local or remote, instead of the archives
//     themselves.
//
// [RenderOptions] decide how each file added to a Stream renders:
//
//   - [RenderOptions.Head] / [RenderOptions.Tail] choose how many lines to
//     include from start/end; a negative Head renders the whole file, and
//     zero for both only its header.
//   - [RenderOptions.MaxSize] skips files larger than the given byte count,
//     and bounds how much of a stream (a FIFO or device) is read; zero skips
//     nothing.
//   - [RenderOptions.LineNumbers] enables numbered output.
//   - [RenderOptions.SkeletonFunctions] / [RenderOptions.SkeletonTypes]
//     reduce source files to signatures/defs.
//   - [RenderOptions.ExtractDocuments] converts .pdf/.docx/.xlsx/.pptx to
//     text and HTML to Markdown.
//   - [RenderOptions.ExtractMedia] replaces audio and video files with their
//     container metadata.
//
// Configuration:
//
//   - [Config.OutputFormat] picks the format's profile: "markdown", "xml",
//     "html", or "bare". The profile supplies every template and layout
//     property the Config leaves unset.
//   - The template fields, such as [Config.FileContentTemplate] and
//     [Config.SectionTemplate], replace the profile's templates. Each renders
//     a context: [FileContext], [SectionContext], [PromptContext],
//     [TreeContext], [MetaContext], [HeaderContext] or [FooterContext], all
//     of which reach the [Plan] as .Global.
//   - [Config.Separators] and [Config.EmbedImages], when set, replace the
//     profile's layout: blank lines between items, and images read into
//     [FileContext.DataURI] for [Config.FileImageTemplate].
//
// Stream methods:
//
//   - [Stream.AddFile], [Stream.AddSection], [Stream.AddPrompt] and
//     [Stream.AddMeta] enqueue content.
//   - [Stream.AddFailure] enqueues an input that could not be read, such as
//     one [Collector.Collect] returned, to be reported and counted in output
//     order.
//   - [Stream.Admit] reports whether [Stream.AddFile] keeps a file under its
//     [RenderOptions].
//   - [Stream.AddTree] enqueues a directory tree of the files Admit keeps.
//   - [Stream.Plan] reports what the stream would render, as its templates
//     see it.
//   - [Stream.Execute] renders output to an io.Writer once and returns a
//     [Report] of what it rendered: rows, binary and failed files, bytes and
//     tokens. A Stream renders once: a second Execute returns [ErrExecuted],
//     and adding to or configuring it once Execute has started panics with
//     it, from a [FailureHandler] too.
//   - [Stream.WithConcurrency], [Stream.WithOnFailure], [Stream.WithLogger],
//     [Stream.WithDir] and [Stream.WithTokenizer] customize processing.
//     Skeleton rendering caps the workers at two.
//
// Every input that could not be read is a [Failure], which reaches the caller
// once: [Collector.Collect] returns the ones discovery meets, and a Stream
// hands its handler, in output order, the ones added to it and the files it
// cannot read, counting each in [Report.FailedFiles]. A Stream with no
// handler logs them.
//
// The library logs to the *slog.Logger it is given: [NewCollector],
// [UserIgnoreRules] and [Stream.WithLogger] take one, and nil means
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
