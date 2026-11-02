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

// FileErrorHandler handles file read errors during render.
type FileErrorHandler = render.FileErrorHandler

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
	onFileError FileErrorHandler
	concurrency int
	prepared    []render.PreparedItem

	skippedBySize int
	skeletons     bool
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

func (s *Stream) WithOnFileError(h FileErrorHandler) *Stream {
	s.onFileError = h
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
		slog.Debug("Skipped by size limit (-m)", "path", f.Path, "size", f.Size, "max_size", opts.MaxSize)
		s.skippedBySize++
		return s
	}
	if opts.SkeletonFunctions || opts.SkeletonTypes {
		s.skeletons = true
	}
	s.items = append(s.items, render.FileItem{File: f, Opts: opts})
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

// AddTree enqueues a directory tree of the files Admit keeps under opts. The
// files need not be added with AddFile, so a tree can stand in for content.
// A tree with no admitted files is left out.
func (s *Stream) AddTree(files []sources.InputFile, opts core.RenderOptions) *Stream {
	var paths []string
	for _, f := range files {
		if s.Admit(f, opts) {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) == 0 {
		return s
	}
	s.items = append(s.items, core.TreeContext{Body: buildTree(paths)})
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
