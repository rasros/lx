package lx

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
)

type StreamItem interface {
	isStreamItem()
}

func (InputFile) isStreamItem()      {}
func (SectionContext) isStreamItem() {}
func (PromptContext) isStreamItem()  {}

type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}

type defaultTokenizer struct{}

func (defaultTokenizer) Estimate(size int64, _ interface{}) int64 { return size / 4 }

// preparedItem wraps a raw StreamItem with calculated metadata
type preparedItem struct {
	raw              StreamItem
	section          *SectionContext
	fileIndexGlobal  int
	fileIndexSection int
	streamIndex      int // Order in the output
}

type Stream struct {
	items     []StreamItem
	tokenizer Tokenizer
	engine    *TemplateEngine
	renderCfg RunnerConfig
	workDir   string
	format    string

	// Calculated state
	finalStats    *GlobalContext
	preparedItems []preparedItem
	sections      []*SectionContext
}

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
		engine:    engine,
		renderCfg: runnerCfg,
		tokenizer: defaultTokenizer{},
		workDir:   ".",
		format:    fmtType,
	}, nil
}

func (s *Stream) WithTokenizer(t Tokenizer) *Stream {
	s.tokenizer = t
	return s
}

func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.renderCfg = cfg
	return s
}

func (s *Stream) AddFile(f InputFile) *Stream {
	f.Config = s.renderCfg
	s.items = append(s.items, f)
	return s
}

func (s *Stream) AddSection(title string) *Stream {
	// Add as a placeholder item. Prepare() will refine this.
	s.items = append(s.items, SectionContext{Body: title})
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text})
	return s
}

func (s *Stream) Prepare() GlobalContext {
	if s.finalStats != nil {
		return *s.finalStats
	}

	global := GlobalContext{
		WorkDir:  s.workDir,
		Metadata: make(map[string]string),
	}

	// 1. Initial Pass: Identify Sections and assign items
	s.sections = make([]*SectionContext, 0)
	s.preparedItems = make([]preparedItem, 0, len(s.items))

	// Create an implicit root section
	currentSection := &SectionContext{
		Index:      0,
		Body:       "",
		IsImplicit: true,
	}
	// We only add it to s.sections if we actually use it (or if it's the only one)
	usingImplicit := true

	// Check if the very first item is a Section definition.
	// If so, we skip the implicit section entirely.
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
		sectionCounter = -1 // Will increment to 0 on first item
	}

	globalFileIdx := 1

	for i, item := range s.items {
		// Handle explicit Section creation
		if sec, ok := item.(SectionContext); ok {
			sectionCounter++
			newSec := &SectionContext{
				Index:      sectionCounter,
				Body:       sec.Body,
				IsImplicit: false,
			}
			s.sections = append(s.sections, newSec)
			currentSection = newSec

			// The SectionContext item itself is added to the stream
			// It belongs to the *new* section conceptually
			s.preparedItems = append(s.preparedItems, preparedItem{
				raw:         item,
				section:     currentSection,
				streamIndex: i,
			})
			continue
		}

		// Handle Files
		if f, ok := item.(InputFile); ok {
			currentSection.TotalFiles++
			currentSection.TotalSize += f.Size
			global.TotalFiles++
			global.TotalSize += f.Size
			global.TokenEstimate += s.tokenizer.Estimate(f.Size, nil)

			s.preparedItems = append(s.preparedItems, preparedItem{
				raw:              item,
				section:          currentSection,
				fileIndexGlobal:  globalFileIdx,
				fileIndexSection: currentSection.TotalFiles,
				streamIndex:      i,
			})
			globalFileIdx++
			continue
		}

		// Handle Prompts
		s.preparedItems = append(s.preparedItems, preparedItem{
			raw:         item,
			section:     currentSection,
			streamIndex: i,
		})
	}

	global.TotalSections = len(s.sections)

	// Backfill global context into sections
	for _, sec := range s.sections {
		sec.Global = global
	}

	return global
}

func (s *Stream) GetGlobalContext() GlobalContext {
	return s.Prepare()
}

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

// pipeline types
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
	sectionIndex int // To track boundaries
}

// bufferPool reduces memory allocation for Output buffers
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// readPool reduces memory allocation for Input (Read) buffers
var readPool = sync.Pool{
	New: func() interface{} {
		// 32KB for EstimateLineCount sampling
		b := make([]byte, 32*1024)
		return &b
	},
}

func (s *Stream) executePipeline(ctx context.Context, dest *byteCounter, global GlobalContext) error {
	numWorkers := runtime.NumCPU()
	jobsCh := make(chan seqJob, numWorkers)
	resultsCh := make(chan result, numWorkers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	// A. Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsCh {
				if ctx.Err() != nil {
					return
				}

				proc := NewProcessor(s.engine, global)
				proc.tokenCounter = s.tokenizer.Estimate

				readBufPtr := readPool.Get().(*[]byte)
				readBuf := *readBufPtr
				buf := bufferPool.Get().(*bytes.Buffer)
				buf.Reset()
				localCounter := &byteCounter{w: buf}

				// Pass the ENTIRE prepared item to Render
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

	// B. Feeder
	go func() {
		for _, item := range s.preparedItems {
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

	// C. Assembler
	nextSeq := 0
	buffer := make(map[int]result)

	hasRenderedFirst := false
	lastWasCompact := false
	useSeparators := s.format != "html" // HTML layout handles spacing via CSS/Tags usually

	// Section Tracking
	// Start with -1 to force a header check on the first item
	currentSectionIndex := -1

	renderSectionBoundary := func(newIndex int) error {
		// If we are already inside a section (current >= 0), close it
		if currentSectionIndex >= 0 {
			// Find context for the closing section
			var ctx SectionContext
			for _, s := range s.sections {
				if s.Index == currentSectionIndex {
					ctx = *s
					break
				}
			}
			if err := s.engine.SectionFooter.Execute(dest, ctx); err != nil {
				return err
			}
		}

		// Open the new section
		currentSectionIndex = newIndex
		var ctx SectionContext
		for _, s := range s.sections {
			if s.Index == newIndex {
				ctx = *s
				break
			}
		}

		// If explicit section title templates are used, they are items in the stream.
		// SectionHeaderTemplate wraps the *files* primarily.
		if err := s.engine.SectionHeader.Execute(dest, ctx); err != nil {
			return err
		}
		return nil
	}

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

			// 1. Check Section Boundary
			if next.sectionIndex != currentSectionIndex {
				if hasRenderedFirst && useSeparators {
					if _, err := dest.Write([]byte("\n\n")); err != nil {
						return err
					}
				}

				if err := renderSectionBoundary(next.sectionIndex); err != nil {
					return err
				}
				// When crossing sections, we might want to reset spacing to avoid double separators
				// after the Header has been printed.
				hasRenderedFirst = false
			}

			// 2. Render Separator (if needed)
			// Don't print separator if we just printed a Section Header (hasRenderedFirst=false reset above)
			if hasRenderedFirst && useSeparators {
				sep := "\n\n"
				if lastWasCompact && next.isCompact {
					sep = "\n"
				}
				if _, err := dest.Write([]byte(sep)); err != nil {
					return err
				}
			}

			// 3. Render Item Content
			if _, err := dest.Write(next.buffer.Bytes()); err != nil {
				return err
			}

			hasRenderedFirst = true
			lastWasCompact = next.isCompact
			bufferPool.Put(next.buffer)
			delete(buffer, nextSeq)
			nextSeq++
		}
	}

	// Close final section
	if currentSectionIndex >= 0 {
		var ctx SectionContext
		for _, s := range s.sections {
			if s.Index == currentSectionIndex {
				ctx = *s
				break
			}
		}
		if err := s.engine.SectionFooter.Execute(dest, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Stream) GetEngine() *TemplateEngine { return s.engine }

// byteCounter wraps an io.Writer to track total bytes written
type byteCounter struct {
	w     io.Writer
	count int64
}

func (b *byteCounter) Write(p []byte) (n int, err error) {
	n, err = b.w.Write(p)
	b.count += int64(n)
	return
}
