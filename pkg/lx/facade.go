package lx

import (
	"context"
	"io"
	"io/fs"
	"log/slog"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/discover"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Plan is what a Stream is about to render, the .Global of its templates.
type Plan = core.Plan

// Report is what Stream.Execute rendered: its Plan and what only rendering
// finds out.
type Report = core.Report
type RenderOptions = core.RenderOptions
type Config = core.Config
type SectionContext = core.SectionContext
type PromptContext = core.PromptContext

type InputFile = sources.InputFile
type DiscoverOptions = discover.Options

// Glob is one -i or -e glob in DiscoverOptions.Globs, of a GlobKind.
type Glob = walker.Override
type GlobKind = walker.OverrideKind

const (
	// GlobInclude keeps what it matches, as ripgrep's -g glob, and once
	// there is one a file must be kept by some glob.
	GlobInclude = walker.Include
	// GlobExclude drops what it matches, as ripgrep's -g '!glob'.
	GlobExclude = walker.Exclude
	// GlobKeep keeps what it matches without requiring that of every file.
	GlobKeep = walker.Keep
)

type Collector = discover.Collector
type URLError = discover.URLError

// Failure is an input that could not be read, so it is missing from the
// output, of a FailureKind. Collector.Collect returns them; added to a Stream
// with AddFailure they are reported and counted with the files the Stream
// cannot read.
type Failure = core.Failure
type FailureKind = core.FailureKind

const (
	FailedStat         = core.FailedStat
	FailedURL          = core.FailedURL
	FailedWalk         = core.FailedWalk
	FailedArchive      = core.FailedArchive
	FailedArchiveEntry = core.FailedArchiveEntry
	FailedRead         = core.FailedRead
)

type FailureHandler = streamingpkg.FailureHandler

func NewConfig() *Config { return core.NewConfig() }

func Merge(dst *Config, src *Config) { core.Merge(dst, src) }

// NewCollector returns a Collector, which finds the inputs of the targets of
// one run as lx does. Relative targets resolve against cwd, or the process's
// working directory when it is empty. Collect returns the inputs it fails
// on; the rest is logged to log, or slog.Default() when it is nil.
func NewCollector(cwd string, log *slog.Logger) *Collector { return discover.NewCollector(cwd, log) }

// IncludeGlob returns lx's -i pattern p, ripgrep's -g p: a GlobInclude, or a
// GlobExclude of what follows a leading "!". Glob.Validate reports a glob
// that cannot be parsed.
func IncludeGlob(p string) Glob { return walker.IncludeOverride(p) }

// ExcludeGlob returns lx's -e pattern p, ripgrep's -g '!p': a GlobExclude,
// or a GlobKeep of what follows a leading "!".
func ExcludeGlob(p string) Glob { return walker.ExcludeOverride(p) }

// UserIgnoreRules reads the ignore files lx applies everywhere, for
// DiscoverOptions.IgnoreRules and GitExcludes, finding them through env, as
// os.LookupEnv looks variables up. It logs to log, or slog.Default() when nil.
func UserIgnoreRules(env func(key string) (string, bool), log *slog.Logger) (ignoreRules, gitExcludes []string) {
	return discover.UserIgnoreRules(env, log)
}

func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	return sources.NewInputFile(fsys, path, info)
}

func NewBufferInputFile(name string, data []byte) InputFile {
	return sources.NewBufferInputFile(name, data)
}

type Stream struct {
	inner *streamingpkg.Stream
}

func NewStream(cfg *Config) (*Stream, error) {
	s, err := streamingpkg.NewStream(cfg)
	if err != nil {
		return nil, err
	}
	return &Stream{inner: s}, nil
}

// WithConcurrency sets the concurrent workers for file processing. The stream
// lowers it to 2 while any queued file renders as a skeleton, because
// tree-sitter parsing does not scale past two workers.
func (s *Stream) WithConcurrency(n int) *Stream {
	s.inner.WithConcurrency(n)
	return s
}

// WithOnFailure sets h to be told about every failure, in output order,
// from the goroutine running Execute: the ones added with AddFailure and the
// files that cannot be read as they render, each counted once in the
// Report's FailedFiles. Without one, the stream logs them.
func (s *Stream) WithOnFailure(h FailureHandler) *Stream {
	s.inner.WithOnFailure(h)
	return s
}

// AddFailure adds an input that could not be read, such as one Collect
// returned. It renders nothing, but is reported where it stands in the output
// and counted in the Report's FailedFiles.
func (s *Stream) AddFailure(f Failure) *Stream {
	s.inner.AddFailure(f)
	return s
}

// WithDir sets what relative paths in templates, such as dataURI's, resolve
// against; the default is the process's working directory.
func (s *Stream) WithDir(dir string) *Stream {
	s.inner.WithDir(dir)
	return s
}

// WithLogger sets where the stream logs; nil means slog.Default().
func (s *Stream) WithLogger(log *slog.Logger) *Stream {
	s.inner.WithLogger(log)
	return s
}

// Admit reports whether the stream keeps f when added with opts: a file over
// opts.MaxSize is dropped. AddFile and AddTree both decide by it.
func (s *Stream) Admit(f InputFile, opts RenderOptions) bool { return s.inner.Admit(f, opts) }

// AddFile enqueues f to render with opts, unless Admit drops it.
func (s *Stream) AddFile(f InputFile, opts RenderOptions) *Stream {
	s.inner.AddFile(f, opts)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	s.inner.AddSection(title)
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.inner.AddPrompt(text)
	return s
}

// AddTree enqueues a directory tree of the files Admit keeps under opts; only
// opts.MaxSize applies. The files need not be added with AddFile, so a tree can
// stand in for content. A tree with no admitted files is left out.
func (s *Stream) AddTree(files []InputFile, opts RenderOptions) *Stream {
	s.inner.AddTree(files, opts)
	return s
}

func (s *Stream) AddMeta(body string, fields map[string]string) *Stream {
	s.inner.AddMeta(body, fields)
	return s
}

// Plan returns what the stream would render now, as its templates will see
// it.
func (s *Stream) Plan() Plan { return s.inner.Plan() }

// ErrExecuted is returned by Execute on a stream that already ran, and the
// panic of a method that changes a stream once Execute has started.
var ErrExecuted = streamingpkg.ErrExecuted

// Execute renders the stream to w and reports what was rendered. A stream
// renders once: every later call returns ErrExecuted.
func (s *Stream) Execute(ctx context.Context, w io.Writer) (Report, error) {
	return s.inner.Execute(ctx, w)
}
