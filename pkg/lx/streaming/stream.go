package streaming

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/render"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

// FailureHandler is told about each input that could not be read. It is
// called from the goroutine running Execute, in output order. It may read the
// stream's Plan, but changing the stream from it panics, as it does after
// Execute.
type FailureHandler func(core.Failure)

// Stream manages a collection of files, sections, and prompts for rendering.
// It is not safe for concurrent use: build it and execute it from one
// goroutine. It renders once: once Execute has started, the methods that
// change the stream panic with ErrExecuted, and Execute returns it.
type Stream struct {
	items       []interface{}
	tokenizer   core.Tokenizer
	engine      *core.TemplateEngine
	workDir     string
	executed    bool
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
		tokenizer:   core.DefaultTokenizer(),
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
	s.mustChange()
	if n < 1 {
		n = 1
	}
	s.concurrency = n
	return s
}

func (s *Stream) WithTokenizer(t core.Tokenizer) *Stream {
	s.mustChange()
	s.tokenizer = t
	return s
}

// WithOnFailure sets h to be told about every failure: the ones added with
// AddFailure and the files that cannot be read as they render, each counted
// once in the Report's FailedFiles. Without one, the stream logs them.
func (s *Stream) WithOnFailure(h FailureHandler) *Stream {
	s.mustChange()
	s.onFailure = h
	return s
}

// WithDir sets what relative paths in templates, such as dataURI's, resolve
// against; the default is the process's working directory.
func (s *Stream) WithDir(dir string) *Stream {
	s.mustChange()
	s.engine.Dir = dir
	return s
}

// WithLogger sets where the stream logs; the default is slog.Default().
func (s *Stream) WithLogger(log *slog.Logger) *Stream {
	s.mustChange()
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
	s.mustChange()
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
// output and counted in the Report's FailedFiles.
func (s *Stream) AddFailure(f core.Failure) *Stream {
	s.mustChange()
	s.items = append(s.items, f)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	s.mustChange()
	s.items = append(s.items, core.SectionContext{Body: title})
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.mustChange()
	s.items = append(s.items, core.PromptContext{Body: text})
	return s
}

// AddTree enqueues a directory tree of the files Admit keeps under opts; only
// opts.MaxSize applies. The files need not be added with AddFile, so a tree can
// stand in for content. A tree with no admitted files is left out. Repeated
// calls with the same files slice and MaxSize share one built tree.
func (s *Stream) AddTree(files []sources.InputFile, opts core.RenderOptions) *Stream {
	s.mustChange()
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
	s.mustChange()
	s.items = append(s.items, core.MetaContext{Body: body, Fields: fields})
	return s
}

// mustChange panics once Execute has started: a stream renders once, and
// nothing may change it while it renders.
func (s *Stream) mustChange() {
	if s.executed {
		panic(ErrExecuted)
	}
}

// prepare plans the items queued so far, organizing them into sections. It
// only reads the stream.
func (s *Stream) prepare() (core.Plan, []*core.SectionContext, []render.PreparedItem) {
	global := core.Plan{
		WorkDir:       s.workDir,
		SkippedBySize: s.skippedBySize,
	}

	sections := make([]*core.SectionContext, 0)
	prepared := make([]render.PreparedItem, 0, len(s.items))

	currentSection := &core.SectionContext{Index: 0, Body: "", IsImplicit: true}
	usingImplicit := true
	if len(s.items) > 0 {
		if _, ok := s.items[0].(core.SectionContext); ok {
			usingImplicit = false
		}
	}
	if usingImplicit {
		sections = append(sections, currentSection)
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
			sections = append(sections, newSec)
			currentSection = newSec
			prepared = append(prepared, render.PreparedItem{Raw: item, Section: currentSection, StreamIndex: i})
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
			// Size-only: no content read yet. The Report adds WrittenTokenEstimate,
			// over the written output, beside it.
			global.TokenEstimate += s.tokenizer.Estimate(f.Size, nil)

			prepared = append(prepared, render.PreparedItem{
				Raw:              item,
				Section:          currentSection,
				FileIndexGlobal:  globalFileIdx,
				FileIndexSection: currentSection.TotalFiles,
				StreamIndex:      i,
			})
			globalFileIdx++
			continue
		}

		prepared = append(prepared, render.PreparedItem{Raw: item, Section: currentSection, StreamIndex: i})
	}

	global.TotalSections = len(sections)
	for _, sec := range sections {
		sec.Global = global
	}

	return global, sections, prepared
}

// Plan returns what the stream would render now, as templates will see it.
func (s *Stream) Plan() core.Plan {
	plan, _, _ := s.prepare()
	return plan
}

// ErrExecuted is returned by Execute on a stream that already ran, and the
// panic of a change to the stream once Execute has started.
var ErrExecuted = errors.New("stream already executed")

// Execute renders the stream to w and reports what was rendered. A stream
// renders once: every later call returns ErrExecuted.
func (s *Stream) Execute(ctx context.Context, w io.Writer) (core.Report, error) {
	if s.executed {
		return core.Report{}, ErrExecuted
	}
	s.executed = true

	plan, sections, prepared := s.prepare()
	s.sections, s.prepared = sections, prepared
	counter := &byteCounter{w: w}

	headerTokens, err := render.ExecuteCounted(counter, s.engine.OutputHeader, core.HeaderContext{Global: plan}, s.tokenizer)
	if err != nil {
		return core.Report{}, err
	}
	totals, err := s.executePipeline(ctx, counter, plan)
	if err != nil {
		return core.Report{}, err
	}
	footerTokens, err := render.ExecuteCounted(counter, s.engine.OutputFooter, core.FooterContext{Global: plan}, s.tokenizer)
	if err != nil {
		return core.Report{}, err
	}

	return core.Report{
		Plan:                 plan,
		TotalWrittenBytes:    counter.count,
		TotalRows:            totals.rows,
		WrittenTokenEstimate: headerTokens + totals.tokens + footerTokens,
		BinaryFiles:          totals.binary,
		FailedFiles:          totals.failed,
	}, nil
}
