package lx

import (
	"context"
	"io"
)

// StreamItem is the common interface for things that can be rendered into the prompt stream.
type StreamItem interface {
	isStreamItem()
}

func (InputFile) isStreamItem()      {}
func (SectionContext) isStreamItem() {}
func (PromptContext) isStreamItem()  {}

// Tokenizer defines how tokens are calculated for an item.
type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}

type defaultTokenizer struct{}

func (defaultTokenizer) Estimate(size int64, _ interface{}) int64 { return size / 4 }

// Stream is a builder and executor for constructing LLM context prompts.
type Stream struct {
	items     []StreamItem
	tokenizer Tokenizer
	engine    *TemplateEngine
	renderCfg RunnerConfig
	workDir   string
}

// NewStream initializes a new prompt stream with the provided configuration.
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

// WithTokenizer allows overriding the default character-based token estimation.
func (s *Stream) WithTokenizer(t Tokenizer) *Stream {
	s.tokenizer = t
	return s
}

// WithRunnerConfig updates the slicing/formatting state for subsequent files added via programmatic loops.
func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.renderCfg = cfg
	return s
}

// AddFile appends a file to the stream.
func (s *Stream) AddFile(f InputFile) *Stream {
	s.items = append(s.items, f)
	return s
}

// AddSection appends a visual section header to the stream.
func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, SectionContext{Body: title})
	return s
}

// AddPrompt appends custom instructions or text to the stream.
func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text})
	return s
}

// Prepare calculates aggregate statistics across all items currently in the stream.
func (s *Stream) Prepare() GlobalContext {
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

// GetGlobalContext returns the current statistics of the stream.
func (s *Stream) GetGlobalContext() GlobalContext {
	return s.Prepare()
}

// Execute renders the entire stream to the provided writer.
// It automatically calls Prepare() to ensure templates have access to accurate global totals.
func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	global := s.Prepare()
	proc := NewProcessor(s.engine, s.renderCfg, global)
	proc.tokenCounter = s.tokenizer.Estimate

	// 1. Render Header
	if err := s.engine.Header.Execute(w, HeaderContext{Global: global}); err != nil {
		return err
	}

	// 2. Render Items
	fileIdx := 1
	for _, item := range s.items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := proc.Render(w, item, fileIdx); err != nil {
				return err
			}
			if _, ok := item.(InputFile); ok {
				fileIdx++
			}
		}
	}

	// 3. Render Footer
	return s.engine.Footer.Execute(w, FooterContext{Global: global})
}

func (s *Stream) GetEngine() *TemplateEngine { return s.engine }
