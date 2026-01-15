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

type Stream struct {
	items      []StreamItem
	tokenizer  Tokenizer
	engine     *TemplateEngine
	renderCfg  RunnerConfig
	workDir    string
	format     string // "markdown", "xml", "html"
	finalStats *GlobalContext
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

	ctx := GlobalContext{
		WorkDir:  s.workDir,
		Metadata: make(map[string]string),
	}

	sectionCount := 0
	for _, item := range s.items {
		switch v := item.(type) {
		case InputFile:
			ctx.TotalFiles++
			ctx.TotalSize += v.Size
			ctx.TokenEstimate += s.tokenizer.Estimate(v.Size, nil)
		case SectionContext:
			sectionCount++
		}
	}
	ctx.TotalSections = sectionCount
	return ctx
}

func (s *Stream) GetGlobalContext() GlobalContext {
	return s.Prepare()
}

// Execute processes the stream using a parallel pipeline to maximize I/O and CPU usage
// while maintaining strict output order.
func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	global := s.Prepare()

	// Wrap writer to track actual bytes written for stats
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

	// Update the cached stats with the actual written byte count
	global.TotalWrittenBytes = counter.count
	s.finalStats = &global

	return nil
}

// pipeline types
type seqJob struct {
	seqID   int
	fileIdx int
	item    StreamItem
}

type result struct {
	index     int
	buffer    *bytes.Buffer // Holds pooled buffer to reduce GC pressure
	stats     int64
	isCompact bool
	err       error
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
		// 32KB is optimal for EstimateLineCount sampling
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

	// A. Start Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := range jobsCh {
				if ctx.Err() != nil {
					return
				}

				// Create a FRESH processor for every item to reset internal state.
				proc := NewProcessor(s.engine, global)
				proc.tokenCounter = s.tokenizer.Estimate

				readBufPtr := readPool.Get().(*[]byte)
				readBuf := *readBufPtr

				buf := bufferPool.Get().(*bytes.Buffer)
				buf.Reset()

				// Use a local counter to track stats for this specific item
				localCounter := &byteCounter{w: buf}

				err := proc.Render(localCounter, j.item, j.fileIdx, readBuf)

				readPool.Put(readBufPtr)

				select {
				case resultsCh <- result{
					index:     j.seqID,
					buffer:    buf,
					stats:     localCounter.count,
					isCompact: proc.lastWasCompact, // Access internal state of processor
					err:       err,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// B. Start Feeder
	go func() {
		fileIdx := 1
		for i, item := range s.items {
			select {
			case jobsCh <- seqJob{seqID: i, fileIdx: fileIdx, item: item}:
				if _, ok := item.(InputFile); ok {
					fileIdx++
				}
			case <-ctx.Done():
				break
			}
		}
		close(jobsCh)
		wg.Wait()
		close(resultsCh)
	}()

	// C. Start Assembler
	// Buffers out-of-order results and writes them strictly in sequence.
	nextSeq := 0
	buffer := make(map[int]result)

	// State for separator logic (replaces Processor's internal state tracking)
	hasRenderedFirst := false
	lastWasCompact := false

	// HTML templates don't use separators, so we skip injection for them
	useSeparators := s.format != "html"

	for res := range resultsCh {
		if res.err != nil {
			return res.err
		}

		buffer[res.index] = res

		// Drain buffer as much as possible
		for {
			next, ok := buffer[nextSeq]
			if !ok {
				break
			}

			if hasRenderedFirst && useSeparators {
				sep := "\n\n"
				if lastWasCompact && next.isCompact {
					sep = "\n"
				}
				if _, err := dest.Write([]byte(sep)); err != nil {
					return err
				}
			}

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
