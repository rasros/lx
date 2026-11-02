package lx

import (
	"context"
	"io"
	"io/fs"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/discover"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
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
type Collector = discover.Collector
type URLError = discover.URLError

type FileErrorHandler = streamingpkg.FileErrorHandler

func NewConfig() *Config { return core.NewConfig() }

func Merge(dst *Config, src *Config) { core.Merge(dst, src) }

// NewCollector returns a Collector, which finds the inputs of the targets of
// one run as lx does.
func NewCollector() *Collector { return discover.NewCollector() }

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

// AddTree enqueues a directory tree of the files among files that Admit keeps
// under opts. A tree with no admitted files is left out.
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
