package streaming

import (
	"context"
	"io"
	"log/slog"
	"runtime"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/render"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

// FailureHandler is told about each input that could not be read. It is
// called from the goroutine running Execute, in output order.
type FailureHandler func(core.Failure)

type defaultTokenizer struct{}

func (defaultTokenizer) Estimate(size int64, content interface{}) int64 {
	return core.DefaultTokenCounter(size, content)
}

// Stream manages a collection of files, sections, and prompts for rendering.
type Stream struct {
	items       []interface{}
	tokenizer   core.Tokenizer
	engine      *core.TemplateEngine
	workDir     string
	finalStats  *core.GlobalContext
	sections    []*core.SectionContext
	onFailure   FailureHandler
	log         *slog.Logger
	concurrency int
	prepared    []render.PreparedItem

	skippedBySize int
	skeletons     bool
	lastTree      builtTree
}

// builtTree remembers the last tree AddTree built, so tree ops over the same
// files share it.
type builtTree struct {
	files    []sources.InputFile
	maxSize  int64
	admitted int
	body     string
}

// workers is the worker count for executing the queued files.
func (s *Stream) workers() int {
	if s.skeletons && s.concurrency > maxSkeletonWorkers {
		return maxSkeletonWorkers
	}
	return s.concurrency
}

func NewStream(cfg *core.Config) (*Stream, error) {
	engine, err := templatex.Compile(cfg)
	if err != nil {
		return nil, err
	}

	return &Stream{
		engine:      engine,
		tokenizer:   defaultTokenizer{},
		workDir:     ".",
		concurrency: runtime.NumCPU(),
		log:         slog.Default(),
	}, nil
}

// maxSkeletonWorkers caps the workers while any queued file renders as a
// skeleton, because tree-sitter parsing does not scale past two workers.
const maxSkeletonWorkers = 2

// WithConcurrency sets the concurrent workers for file processing. The stream
// lowers it to maxSkeletonWorkers when any queued file renders as a skeleton.
func (s *Stream) WithConcurrency(n int) *Stream {
	if n < 1 {
		n = 1
	}
	s.concurrency = n
	return s
}

func (s *Stream) WithTokenizer(t core.Tokenizer) *Stream {
	s.tokenizer = t
	return s
}

// WithOnFailure sets h to be told about every failure: the ones added with
// AddFailure and the files that cannot be read as they render, each counted
// once in FailedFiles. Without one, the stream logs them.
func (s *Stream) WithOnFailure(h FailureHandler) *Stream {
	s.onFailure = h
	return s
}

// WithDir sets what relative paths in templates, such as dataURI's, resolve
// against; the default is the process's working directory.
func (s *Stream) WithDir(dir string) *Stream {
	s.engine.Dir = dir
	return s
}

// WithLogger sets where the stream logs; the default is slog.Default().
func (s *Stream) WithLogger(log *slog.Logger) *Stream {
	if log == nil {
		log = slog.Default()
	}
	s.log = log
	return s
}

// Admit reports whether the stream keeps f when added with opts: a file over
// opts.MaxSize is dropped. Inputs with an unknown size (negative, as for
// remote URLs) are always kept. AddFile and AddTree both decide by it.
func (s *Stream) Admit(f sources.InputFile, opts core.RenderOptions) bool {
	return opts.MaxSize <= 0 || f.Size <= opts.MaxSize
}

// AddFile enqueues a file to render with opts, unless Admit drops it.
func (s *Stream) AddFile(f sources.InputFile, opts core.RenderOptions) *Stream {
	if !s.Admit(f, opts) {
		s.log.Debug("Skipped by size limit", "path", f.Path, "size", f.Size, "max_size", opts.MaxSize)
		s.skippedBySize++
		return s
	}
	if opts.SkeletonFunctions || opts.SkeletonTypes {
		s.skeletons = true
	}
	s.items = append(s.items, render.FileItem{File: f, Opts: opts})
	return s
}

// AddFailure adds an input that could not be read, such as one discovery
// failed on. It renders nothing, but is reported where it stands in the
// output and counted in FailedFiles.
func (s *Stream) AddFailure(f core.Failure) *Stream {
	s.items = append(s.items, f)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, core.SectionContext{Body: title})
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, core.PromptContext{Body: text})
	return s
}

// AddTree enqueues a directory tree of the files Admit keeps under opts; only
// opts.MaxSize applies. The files need not be added with AddFile, so a tree can
// stand in for content. A tree with no admitted files is left out. Repeated
// calls with the same files slice and MaxSize share one built tree.
func (s *Stream) AddTree(files []sources.InputFile, opts core.RenderOptions) *Stream {
	if t := s.lastTree; len(files) > 0 && len(files) == len(t.files) &&
		&files[0] == &t.files[0] && opts.MaxSize == t.maxSize {
		s.log.Debug("Adding tree", "files", t.admitted)
		s.items = append(s.items, core.TreeContext{Body: t.body})
		return s
	}
	var paths []string
	for _, f := range files {
		if s.Admit(f, opts) {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) == 0 {
		s.log.Debug("Skipping empty tree (no admitted files)")
		return s
	}
	s.lastTree = builtTree{files: files, maxSize: opts.MaxSize, admitted: len(paths), body: buildTree(paths)}
	s.log.Debug("Adding tree", "files", len(paths))
	s.items = append(s.items, core.TreeContext{Body: s.lastTree.body})
	return s
}

func (s *Stream) AddMeta(body string, fields map[string]string) *Stream {
	s.items = append(s.items, core.MetaContext{Body: body, Fields: fields})
	return s
}

// Prepare calculates metadata and organizes items into sections before rendering.
func (s *Stream) Prepare() core.GlobalContext {
	if s.finalStats != nil {
		return *s.finalStats
	}

	global := core.GlobalContext{
		WorkDir:       s.workDir,
		Metadata:      make(map[string]string),
		SkippedBySize: s.skippedBySize,
	}

	s.sections = make([]*core.SectionContext, 0)
	s.prepared = make([]render.PreparedItem, 0, len(s.items))

	currentSection := &core.SectionContext{Index: 0, Body: "", IsImplicit: true}
	usingImplicit := true
	if len(s.items) > 0 {
		if _, ok := s.items[0].(core.SectionContext); ok {
			usingImplicit = false
		}
	}
	if usingImplicit {
		s.sections = append(s.sections, currentSection)
	}

	sectionCounter := 0
	if !usingImplicit {
		sectionCounter = -1
	}
	globalFileIdx := 1

	for i, item := range s.items {
		if sec, ok := item.(core.SectionContext); ok {
			sectionCounter++
			newSec := &core.SectionContext{Index: sectionCounter, Body: sec.Body, IsImplicit: false}
			s.sections = append(s.sections, newSec)
			currentSection = newSec
			s.prepared = append(s.prepared, render.PreparedItem{Raw: item, Section: currentSection, StreamIndex: i})
			continue
		}

		if fi, ok := item.(render.FileItem); ok {
			f := fi.File
			currentSection.TotalFiles++
			global.TotalFiles++
			if f.Size < 0 {
				global.SizeUnknown = true
			} else {
				currentSection.TotalSize += f.Size
				global.TotalSize += f.Size
			}
			// Size-only: no content read yet. Execute replaces this.
			global.TokenEstimate += s.tokenizer.Estimate(f.Size, nil)

			s.prepared = append(s.prepared, render.PreparedItem{
				Raw:              item,
				Section:          currentSection,
				FileIndexGlobal:  globalFileIdx,
				FileIndexSection: currentSection.TotalFiles,
				StreamIndex:      i,
			})
			globalFileIdx++
			continue
		}

		s.prepared = append(s.prepared, render.PreparedItem{Raw: item, Section: currentSection, StreamIndex: i})
	}

	global.TotalSections = len(s.sections)
	for _, sec := range s.sections {
		sec.Global = global
	}

	return global
}

func (s *Stream) GetGlobalContext() core.GlobalContext { return s.Prepare() }

func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	global := s.Prepare()
	counter := &byteCounter{w: w}

	headerTokens, err := render.ExecuteCounted(counter, s.engine.OutputHeader, core.HeaderContext{Global: global}, s.tokenizer)
	if err != nil {
		return err
	}
	totals, err := s.executePipeline(ctx, counter, global)
	if err != nil {
		return err
	}
	footerTokens, err := render.ExecuteCounted(counter, s.engine.OutputFooter, core.FooterContext{Global: global}, s.tokenizer)
	if err != nil {
		return err
	}

	global.TotalWrittenBytes = counter.count
	global.TotalRows = totals.rows
	global.BinaryFiles = totals.binary
	global.FailedFiles = totals.failed
	global.TokenEstimate = headerTokens + totals.tokens + footerTokens
	s.finalStats = &global
	return nil
}

func (s *Stream) GetEngine() *core.TemplateEngine { return s.engine }
