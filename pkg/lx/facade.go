package lx

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/discover"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
	"github.com/rasros/lx/pkg/lx/walker"
)

// Config is a Stream's templates and output format, the library keys of lx's
// config file.
type Config = core.Config

// RenderOptions decide how a file added to a Stream renders.
type RenderOptions = core.RenderOptions

// Plan is what a Stream is about to render, the .Global of its templates.
type Plan = core.Plan

// Report is what Stream.Execute rendered: its Plan and what only rendering
// finds out.
type Report = core.Report

// Tokenizer estimates the tokens a Stream reports and templates see.
type Tokenizer = core.Tokenizer

// DefaultTokenizer returns the Tokenizer a Stream uses unless
// Stream.WithTokenizer sets another, for one to wrap or fall back to. It
// scans the text by character class, with no vocabulary.
func DefaultTokenizer() Tokenizer { return core.DefaultTokenizer() }

// The template contexts: what each of a Config's templates renders.
type (
	// FileContext is what the file templates render: FileContentTemplate,
	// FileErrorTemplate, FileBinaryTemplate, FileCompactTemplate,
	// FileImageTemplate and FileHeaderTemplate.
	FileContext = core.FileContext
	// SectionContext is what SectionTemplate, SectionHeaderTemplate and
	// SectionFooterTemplate render.
	SectionContext = core.SectionContext
	// PromptContext is what PromptTemplate renders.
	PromptContext = core.PromptContext
	// TreeContext is what TreeTemplate renders.
	TreeContext = core.TreeContext
	// MetaContext is what MetaTemplate renders.
	MetaContext = core.MetaContext
	// HeaderContext is what OutputHeaderTemplate renders.
	HeaderContext = core.HeaderContext
	// FooterContext is what OutputFooterTemplate renders.
	FooterContext = core.FooterContext
)

// InputFile is a file to render, found by a Collector or made with
// NewInputFile or NewBufferInputFile.
type InputFile = sources.InputFile

// DiscoverOptions decide what Collector.Collect finds.
type DiscoverOptions = discover.Options

// Glob is one -i or -e glob in DiscoverOptions.Globs, of a GlobKind.
type Glob = walker.Override

// GlobKind is what a Glob does with the paths it matches: GlobInclude,
// GlobExclude or GlobKeep.
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

// Collector finds the inputs a path or URL refers to, as lx does.
type Collector = discover.Collector

// URLError is the Err of a Failure of a URL that could not be parsed or
// downloaded.
type URLError = discover.URLError

// Failure is an input that could not be read, so it is missing from the
// output, of a FailureKind. Collector.Collect returns them; added to a Stream
// with AddFailure they are reported and counted with the files the Stream
// cannot read.
type Failure = core.Failure

// FailureKind is the step an input failed at, from FailedStat to
// FailedRead.
type FailureKind = core.FailureKind

const (
	FailedStat         = core.FailedStat
	FailedURL          = core.FailedURL
	FailedWalk         = core.FailedWalk
	FailedArchive      = core.FailedArchive
	FailedArchiveEntry = core.FailedArchiveEntry
	FailedRead         = core.FailedRead
)

// FailureHandler is told about each Failure of a Stream, in output order.
type FailureHandler = streamingpkg.FailureHandler

// NewConfig returns the default Config, which renders Markdown.
func NewConfig() *Config { return core.NewConfig() }

// Merge sets in dst the fields src sets: its non-empty templates and
// strings, and its non-nil layout properties.
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

// NewInputFile returns the file at path in fsys, described by info.
func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	return sources.NewInputFile(fsys, path, info)
}

// NewBufferInputFile returns a file named name holding data.
func NewBufferInputFile(name string, data []byte) InputFile {
	return sources.NewBufferInputFile(name, data)
}

// Stream renders files, sections, prompts, trees and metadata in the order
// they are added. It is not safe for concurrent use, and renders once.
// StreamLimitError is a stream, such as a FIFO, a device or stdin, that went
// on past the limit it is read to.
type StreamLimitError = sources.StreamLimitError

// ReadStream reads r to its end as lx reads a FIFO or a device: up to limit,
// or a 64 MB default when limit is not positive, failing with a
// *StreamLimitError past it.
func ReadStream(r io.Reader, limit int64) ([]byte, error) { return sources.ReadStream(r, limit) }

// WaitCleanups waits up to d for work a cancel abandoned that removes
// temporary files once it finishes, and reports whether all of it finished.
// A process about to exit after a cancel calls it.
func WaitCleanups(d time.Duration) bool { return sources.WaitCleanups(d) }

type Stream struct {
	inner *streamingpkg.Stream
}

// NewStream returns an empty Stream rendering with cfg, or the error of a
// template in it that does not parse or names a field its context lacks.
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

// WithTokenizer sets the estimator of the tokens the stream reports and its
// templates see as FileContext.TokenEstimate; the default is
// DefaultTokenizer.
func (s *Stream) WithTokenizer(t Tokenizer) *Stream {
	s.inner.WithTokenizer(t)
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

// AddSection starts a section titled title; what is added after it belongs
// to it. What is added before the first section belongs to an implicit one.
func (s *Stream) AddSection(title string) *Stream {
	s.inner.AddSection(title)
	return s
}

// AddPrompt enqueues text, rendered with the PromptTemplate.
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

// AddMeta enqueues a metadata block, rendered with the MetaTemplate: body
// as rendered by default and fields for a template to pick from.
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
