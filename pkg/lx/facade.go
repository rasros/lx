package lx

import (
	"context"
	"io"
	"io/fs"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	streamingpkg "github.com/rasros/lx/pkg/lx/streaming"
	"github.com/rasros/lx/pkg/lx/templatex"
	walkerpkg "github.com/rasros/lx/pkg/lx/walker"
)

type GlobalContext = core.GlobalContext
type RunnerConfig = core.RunnerConfig
type TemplateEngine = core.TemplateEngine
type Config = core.Config
type FileContext = core.FileContext
type SectionContext = core.SectionContext
type PromptContext = core.PromptContext
type TreeContext = core.TreeContext
type MetaContext = core.MetaContext
type HeaderContext = core.HeaderContext
type FooterContext = core.FooterContext
type StatsContext = core.StatsContext
type TokenCounter = core.TokenCounter
type Tokenizer = core.Tokenizer

type InputFile = sources.InputFile
type Rule = walkerpkg.Rule
type Walker = walkerpkg.Walker
type CompiledSpec = walkerpkg.CompiledSpec

type FileErrorHandler = streamingpkg.FileErrorHandler

func NewConfig() *Config { return core.NewConfig() }

func Merge(dst *Config, src *Config) { core.Merge(dst, src) }

// CompileTemplates parses the configuration templates into a TemplateEngine.
func CompileTemplates(cfg *Config) (*TemplateEngine, error) { return templatex.Compile(cfg) }

func DefaultTokenCounter(size int64, content interface{}) int64 {
	return core.DefaultTokenCounter(size, content)
}

func templateFuncs() template.FuncMap { return templatex.TemplateFuncs() }

func NewWalker(basePatterns, overridePatterns []string) *Walker {
	return walkerpkg.NewWalker(basePatterns, overridePatterns)
}

func IsMatch(pattern, relPath string) bool { return walkerpkg.IsMatch(pattern, relPath) }

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

func NewInputFileFromPath(fsys fs.FS, path string) (InputFile, error) {
	return sources.NewInputFileFromPath(fsys, path)
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

func IsDocumentPath(path string) bool { return sources.IsDocumentPath(path) }

func ExtractDocumentText(path string, r io.ReaderAt, size int64) ([]byte, error) {
	return sources.ExtractDocumentText(path, r, size)
}

type Stream struct {
	inner *streamingpkg.Stream
	items []interface{}
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

func (s *Stream) WithTokenizer(t Tokenizer) *Stream {
	s.inner.WithTokenizer(t)
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
	s.items = append(s.items, f)
	s.inner.AddFile(f)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, SectionContext{Body: title})
	s.inner.AddSection(title)
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text})
	s.inner.AddPrompt(text)
	return s
}

func (s *Stream) AddTree(body string) *Stream {
	s.inner.AddTree(body)
	return s
}

func (s *Stream) Prepare() GlobalContext { return s.inner.Prepare() }

func (s *Stream) GetGlobalContext() GlobalContext { return s.inner.GetGlobalContext() }

func (s *Stream) Execute(ctx context.Context, w io.Writer) error { return s.inner.Execute(ctx, w) }

func (s *Stream) GetEngine() *TemplateEngine { return s.inner.GetEngine() }

func (s *Stream) setWorkDir(dir string) {
	_ = dir
}

type streamSink struct{ s *Stream }

func (ss streamSink) Add(f InputFile) { ss.s.AddFile(f) }

func ExpandArchive(ctx context.Context, absPath, displayPath string, walker *Walker, includes []string, outPath string, stream *Stream) error {
	return sources.ExpandArchive(ctx, absPath, displayPath, walker, includes, outPath, streamSink{s: stream})
}

func ExpandArchivePaths(ctx context.Context, absPath, displayPath string, walker *Walker, includes []string, maxSize int64) ([]string, error) {
	return sources.ExpandArchivePaths(ctx, absPath, displayPath, walker, includes, maxSize)
}
