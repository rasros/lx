package lx

import (
	"context"
	"io"
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
	finalStats *GlobalContext
}

func NewStream(cfg *Config, runnerCfg RunnerConfig) (*Stream, error) {
	engine, err := CompileTemplates(cfg)
	if err != nil {
		return nil, err
	}
	return &Stream{
		engine:    engine,
		renderCfg: runnerCfg,
		tokenizer: defaultTokenizer{},
		workDir:   ".",
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

func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	global := s.Prepare()

	// Track actual bytes written
	counter := &byteCounter{w: w}

	proc := NewProcessor(s.engine, global)
	proc.tokenCounter = s.tokenizer.Estimate

	if err := s.engine.Header.Execute(counter, HeaderContext{Global: global}); err != nil {
		return err
	}

	fileIdx := 1
	for _, item := range s.items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := proc.Render(counter, item, fileIdx); err != nil {
				return err
			}
			if _, ok := item.(InputFile); ok {
				fileIdx++
			}
		}
	}

	if err := s.engine.Footer.Execute(counter, FooterContext{Global: global}); err != nil {
		return err
	}

	// Update the cached stats with the actual written byte count
	global.TotalWrittenBytes = counter.count
	s.finalStats = &global

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
