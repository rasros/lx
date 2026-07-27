package lx

import (
	"context"
	"io"
	"io/fs"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
	walkerpkg "github.com/rasros/lx/pkg/lx/walker"
)

type GlobalContext = core.GlobalContext
type RunnerConfig = core.RunnerConfig
type TemplateEngine = core.TemplateEngine
type Config = core.Config
type SectionContext = core.SectionContext
type PromptContext = core.PromptContext
type StatsContext = core.StatsContext

type InputFile = sources.InputFile
type Walker = walkerpkg.Walker
type CompiledSpec = walkerpkg.CompiledSpec

type FileErrorHandler = streamingpkg.FileErrorHandler

func NewConfig() *Config { return core.NewConfig() }

func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return walkerpkg.NewWalker(basePatterns, overridePatterns)
}

func CompileSpecs(patterns []string) []CompiledSpec { return walkerpkg.CompileSpecs(patterns) }

func IsMatchAnyCompiled(specs []CompiledSpec, relPath string) bool {
	return walkerpkg.IsMatchAnyCompiled(specs, relPath)
}

func CouldMatchAnyDescendant(specs []CompiledSpec, dirRelPath string) bool {
	return walkerpkg.CouldMatchAnyDescendant(specs, dirRelPath)
}

func IsKept(p string, includes, excludes []string) bool {
	return walkerpkg.IsKept(p, includes, excludes)
}

func NewInputFile(fsys fs.FS, path string, info fs.FileInfo) InputFile {
	return sources.NewInputFile(fsys, path, info)
}

func NewBufferInputFile(name string, data []byte) InputFile {
	return sources.NewBufferInputFile(name, data)
}

func IsHTTPURL(raw string) bool { return sources.IsHTTPURL(raw) }

func IsHTTPArchiveURL(raw string) bool { return sources.IsHTTPArchiveURL(raw) }

func RewriteRepoURL(raw string) (string, bool) { return sources.RewriteRepoURL(raw) }

func NewURLInputFile(raw string) (InputFile, error) { return sources.NewURLInputFile(raw) }

func DownloadURLToTempFile(ctx context.Context, raw string) (path string, cleanup func(), err error) {
	return sources.DownloadURLToTempFile(ctx, raw)
}

func IsArchivePath(path string) bool { return sources.IsArchivePath(path) }

type Stream struct {
	inner *streamingpkg.Stream
}

func NewStream(cfg *Config, runnerCfg RunnerConfig) (*Stream, error) {
	s, err := streamingpkg.NewStream(cfg, runnerCfg)
	if err != nil {
		return nil, err
	}
	return &Stream{inner: s}, nil
}

func (s *Stream) WithConcurrency(n int) *Stream {
	s.inner.WithConcurrency(n)
	return s
}

func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.inner.WithRunnerConfig(cfg)
	return s
}

func (s *Stream) WithOnFileError(h FileErrorHandler) *Stream {
	s.inner.WithOnFileError(h)
	return s
}

func (s *Stream) AddFile(f InputFile) *Stream {
	s.inner.AddFile(f)
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

func (s *Stream) AddTree(body string) *Stream {
	s.inner.AddTree(body)
	return s
}

func (s *Stream) AddMeta(body string, fields map[string]string) *Stream {
	s.inner.AddMeta(body, fields)
	return s
}

func (s *Stream) GetGlobalContext() GlobalContext { return s.inner.GetGlobalContext() }

func (s *Stream) Execute(ctx context.Context, w io.Writer) error { return s.inner.Execute(ctx, w) }

func (s *Stream) GetEngine() *TemplateEngine { return s.inner.GetEngine() }

type streamSink struct{ s *Stream }

func (ss streamSink) Add(f InputFile) { ss.s.AddFile(f) }

func ExpandArchive(ctx context.Context, absPath, displayPath string, walker *Walker, includes []string, outPath string, stream *Stream) error {
	return sources.ExpandArchive(ctx, absPath, displayPath, walker, includes, outPath, streamSink{s: stream})
}

func ExpandArchivePaths(ctx context.Context, absPath, displayPath string, walker *Walker, includes []string, maxSize int64) ([]string, error) {
	return sources.ExpandArchivePaths(ctx, absPath, displayPath, walker, includes, maxSize)
}
