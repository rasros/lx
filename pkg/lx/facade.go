package lx

import (
	"context"
	"io"
	"io/fs"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/discover"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
	"github.com/rasros/lx/pkg/lx/walker"
)

type GlobalContext = core.GlobalContext
type RenderOptions = core.RenderOptions
type TemplateEngine = core.TemplateEngine
type Config = core.Config
type SectionContext = core.SectionContext
type PromptContext = core.PromptContext
type StatsContext = core.StatsContext

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

type FileErrorHandler = streamingpkg.FileErrorHandler

func NewConfig() *Config { return core.NewConfig() }

func Merge(dst *Config, src *Config) { core.Merge(dst, src) }

// NewCollector returns a Collector, which finds the inputs of the targets of
// one run as lx does.
func NewCollector() *Collector { return discover.NewCollector() }

// IncludeGlob returns lx's -i pattern p, ripgrep's -g p: a GlobInclude, or a
// GlobExclude of what follows a leading "!". Glob.Validate reports a glob
// that cannot be parsed.
var IncludeGlob = walker.IncludeOverride

// ExcludeGlob returns lx's -e pattern p, ripgrep's -g '!p': a GlobExclude,
// or a GlobKeep of what follows a leading "!".
var ExcludeGlob = walker.ExcludeOverride

// UserIgnoreRules reads the ignore files lx applies everywhere, for
// DiscoverOptions.IgnoreRules and GitExcludes.
func UserIgnoreRules() (ignoreRules, gitExcludes []string) { return discover.UserIgnoreRules() }

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

func (s *Stream) WithOnFileError(h FileErrorHandler) *Stream {
	s.inner.WithOnFileError(h)
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

func (s *Stream) GetGlobalContext() GlobalContext { return s.inner.GetGlobalContext() }

func (s *Stream) Execute(ctx context.Context, w io.Writer) error { return s.inner.Execute(ctx, w) }

func (s *Stream) GetEngine() *TemplateEngine { return s.inner.GetEngine() }
