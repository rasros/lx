package lx

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
)

// streamItem is an internal interface for types that can be added to the output stream.
type streamItem interface {
	isStreamItem()
}

func (InputFile) isStreamItem()      {}
func (SectionContext) isStreamItem() {}
func (PromptContext) isStreamItem()  {}

// FileErrorHandler is a callback invoked when the processor fails to read a file.
type FileErrorHandler func(f InputFile, err error)

type defaultTokenizer struct{}

func (defaultTokenizer) Estimate(size int64, _ interface{}) int64 { return size / 4 }

type preparedItem struct {
	raw              streamItem
	section          *SectionContext
	fileIndexGlobal  int
	fileIndexSection int
	streamIndex      int
}

// Stream manages a collection of files, sections, and prompts for rendering.
type Stream struct {
	items       []streamItem
	tokenizer   Tokenizer
	engine      *TemplateEngine
	renderCfg   RunnerConfig
	workDir     string
	format      string
	finalStats  *GlobalContext
	sections    []*SectionContext
	onFileError FileErrorHandler
	concurrency int
	prepared    []preparedItem
}

// NewStream initializes a new stream with the given configuration.
func NewStream(cfg *Config, runnerCfg RunnerConfig) (*Stream, error) {
	engine, err := CompileTemplates(cfg)
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

// WithConcurrency sets the number of concurrent workers for file processing.
// Defaults to runtime.NumCPU(). Set to 1 for sequential processing.
func (s *Stream) WithConcurrency(n int) *Stream {
	if n < 1 {
		n = 1
	}
	s.concurrency = n
	return s
}

func (s *Stream) WithTokenizer(t Tokenizer) *Stream {
	s.tokenizer = t
	return s
}

func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.renderCfg = cfg
	return s
}

// WithOnFileError sets the callback for file reading errors.
func (s *Stream) WithOnFileError(h FileErrorHandler) *Stream {
	s.onFileError = h
	return s
}

func (s *Stream) AddFile(f InputFile) *Stream {
	f.Config = s.renderCfg
	s.items = append(s.items, f)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, SectionContext{Body: title})
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text})
	return s
}

// Prepare calculates metadata and organizes items into sections before rendering.
func (s *Stream) Prepare() GlobalContext {
	if s.finalStats != nil {
		return *s.finalStats
	}

	global := GlobalContext{
		WorkDir:  s.workDir,
		Metadata: make(map[string]string),
	}

	s.sections = make([]*SectionContext, 0)
	s.prepared = make([]preparedItem, 0, len(s.items))

	currentSection := &SectionContext{
		Index:      0,
		Body:       "",
		IsImplicit: true,
	}
	usingImplicit := true

	if len(s.items) > 0 {
		if _, ok := s.items[0].(SectionContext); ok {
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
		if sec, ok := item.(SectionContext); ok {
			sectionCounter++
			newSec := &SectionContext{
				Index:      sectionCounter,
				Body:       sec.Body,
				IsImplicit: false,
			}
			s.sections = append(s.sections, newSec)
			currentSection = newSec

			s.prepared = append(s.prepared, preparedItem{
				raw:         item,
				section:     currentSection,
				streamIndex: i,
			})
			continue
		}

		if f, ok := item.(InputFile); ok {
			currentSection.TotalFiles++
			currentSection.TotalSize += f.Size
			global.TotalFiles++
			global.TotalSize += f.Size
			global.TokenEstimate += s.tokenizer.Estimate(f.Size, nil)

			s.prepared = append(s.prepared, preparedItem{
				raw:              item,
				section:          currentSection,
				fileIndexGlobal:  globalFileIdx,
				fileIndexSection: currentSection.TotalFiles,
				streamIndex:      i,
			})
			globalFileIdx++
			continue
		}

		s.prepared = append(s.prepared, preparedItem{
			raw:         item,
			section:     currentSection,
			streamIndex: i,
		})
	}

	global.TotalSections = len(s.sections)

	for _, sec := range s.sections {
		sec.Global = global
	}

	return global
}

func (s *Stream) GetGlobalContext() GlobalContext {
	return s.Prepare()
}

// Execute renders the stream content to w.
func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	global := s.Prepare()
	counter := &byteCounter{w: w}

	if err := s.engine.Header.Execute(counter, HeaderContext{Global: global}); err != nil {
		return err
	}

	if err := s.executePipeline(ctx, counter, global); err != nil {
		return err
	}

	if err := s.engine.Footer.Execute(counter, FooterContext{Global: global}); err != nil {
		return err
	}

	global.TotalWrittenBytes = counter.count
	s.finalStats = &global
	return nil
}

type seqJob struct {
	seqID int
	item  preparedItem
}

type result struct {
	index        int
	buffer       *bytes.Buffer
	stats        int64
	isCompact    bool
	err          error
	sectionIndex int
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

var readPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

func (s *Stream) executePipeline(ctx context.Context, dest *byteCounter, global GlobalContext) error {
	numWorkers := s.concurrency
	jobsCh := make(chan seqJob, numWorkers)
	resultsCh := make(chan result, numWorkers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsCh {
				if ctx.Err() != nil {
					return
				}

				proc := newProcessor(s.engine, global, s.onFileError)
				proc.tokenCounter = s.tokenizer.Estimate

				readBufPtr := readPool.Get().(*[]byte)
				readBuf := *readBufPtr
				buf := bufferPool.Get().(*bytes.Buffer)
				buf.Reset()
				localCounter := &byteCounter{w: buf}

				err := proc.RenderPrepared(localCounter, j.item, readBuf)

				readPool.Put(readBufPtr)

				select {
				case resultsCh <- result{
					index:        j.seqID,
					buffer:       buf,
					stats:        localCounter.count,
					isCompact:    proc.lastWasCompact,
					err:          err,
					sectionIndex: j.item.section.Index,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		for _, item := range s.prepared {
			select {
			case jobsCh <- seqJob{seqID: item.streamIndex, item: item}:
			case <-ctx.Done():
				break
			}
		}
		close(jobsCh)
		wg.Wait()
		close(resultsCh)
	}()

	useSeparators := s.format != "html"
	layout := newLayoutWriter(dest, s.engine, s.sections, useSeparators)
	defer layout.Close()

	nextSeq := 0
	buffer := make(map[int]result)

	for res := range resultsCh {
		if res.err != nil {
			return res.err
		}

		buffer[res.index] = res

		for {
			next, ok := buffer[nextSeq]
			if !ok {
				break
			}

			if err := layout.WriteItem(next); err != nil {
				return err
			}

			bufferPool.Put(next.buffer)
			delete(buffer, nextSeq)
			nextSeq++
		}
	}

	return nil
}

func (s *Stream) GetEngine() *TemplateEngine { return s.engine }

type byteCounter struct {
	w     io.Writer
	count int64
}

func (b *byteCounter) Write(p []byte) (n int, err error) {
	n, err = b.w.Write(p)
	b.count += int64(n)
	return
}
