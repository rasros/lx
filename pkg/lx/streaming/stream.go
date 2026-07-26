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

func (defaultTokenizer) Estimate(size int64, _ interface{}) int64 { return size / 4 }

// Stream manages a collection of files, sections, and prompts for rendering.
type Stream struct {
	items       []interface{}
	tokenizer   core.Tokenizer
	engine      *core.TemplateEngine
	renderCfg   core.RunnerConfig
	workDir     string
	format      string
	finalStats  *core.GlobalContext
	sections    []*core.SectionContext
	onFileError FileErrorHandler
	concurrency int
	prepared    []render.PreparedItem
}

func NewStream(cfg *core.Config, runnerCfg core.RunnerConfig) (*Stream, error) {
	engine, err := templatex.Compile(cfg)
	if err != nil {
		return nil, err
	}

	fmtType := cfg.OutputFormat
	if fmtType == "" {
		fmtType = "markdown"
	}

	return &Stream{
		engine:      engine,
		renderCfg:   runnerCfg,
		tokenizer:   defaultTokenizer{},
		workDir:     ".",
		format:      fmtType,
		concurrency: runtime.NumCPU(),
	}, nil
}

// WithConcurrency sets concurrent workers for file processing.
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

func (s *Stream) WithRunnerConfig(cfg core.RunnerConfig) *Stream {
	s.renderCfg = cfg
	return s
}

func (s *Stream) WithOnFileError(h FileErrorHandler) *Stream {
	s.onFileError = h
	return s
}

// AddFile enqueues a file, unless it exceeds the configured MaxSize. Inputs
// with an unknown size (negative, as for remote URLs) are always kept.
func (s *Stream) AddFile(f sources.InputFile) *Stream {
	f.Config = s.renderCfg
	if s.renderCfg.MaxSize > 0 && f.Size > s.renderCfg.MaxSize {
		slog.Debug("Skipped by size limit (-m)", "path", f.Path, "size", f.Size, "max_size", s.renderCfg.MaxSize)
		return s
	}
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

func (s *Stream) AddTree(body string) *Stream {
	s.items = append(s.items, core.TreeContext{Body: body})
	return s
}

// Prepare calculates metadata and organizes items into sections before rendering.
func (s *Stream) Prepare() core.GlobalContext {
	if s.finalStats != nil {
		return *s.finalStats
	}

	global := core.GlobalContext{
		WorkDir:  s.workDir,
		Metadata: make(map[string]string),
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

		if f, ok := item.(sources.InputFile); ok {
			currentSection.TotalFiles++
			currentSection.TotalSize += f.Size
			global.TotalFiles++
			global.TotalSize += f.Size
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

	if err := s.engine.OutputHeader.Execute(counter, core.HeaderContext{Global: global}); err != nil {
		return err
	}
	totalRows, err := s.executePipeline(ctx, counter, global)
	if err != nil {
		return err
	}
	if err := s.engine.OutputFooter.Execute(counter, core.FooterContext{Global: global}); err != nil {
		return err
	}

	global.TotalWrittenBytes = counter.count
	global.TotalRows = totalRows
	global.TokenEstimate = s.tokenizer.Estimate(counter.count, nil)
	s.finalStats = &global
	return nil
}

func (s *Stream) GetEngine() *core.TemplateEngine { return s.engine }
