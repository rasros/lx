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

type Stream struct {
	processor    *Processor
	items        []StreamItem
	tokenCounter TokenCounter
}

func NewStream(cfg *Config, runnerCfg RunnerConfig) (*Stream, error) {
	engine, err := CompileTemplates(cfg)
	if err != nil {
		return nil, err
	}
	global := GlobalContext{
		WorkDir:  ".",
		Metadata: make(map[string]string),
	}
	return &Stream{
		processor:    NewProcessor(engine, runnerCfg, global),
		tokenCounter: DefaultTokenCounter,
	}, nil
}

// WithTokenCounter allows overriding the default token estimation logic.
func (s *Stream) WithTokenCounter(tc TokenCounter) *Stream {
	s.tokenCounter = tc
	return s
}

func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.processor.cfg = cfg
	return s
}

func (s *Stream) AddFile(f InputFile) *Stream { s.items = append(s.items, f); return s }

func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, SectionContext{Body: title})
	return s
}

func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text})
	return s
}

func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	// Preparation: Sync totals and global context into items
	s.Prepare()

	if err := s.processor.engine.Header.Execute(w, HeaderContext{Global: s.processor.global}); err != nil {
		return err
	}

	fileIdx := 1
	for _, item := range s.items {
		// Ensure current global state is passed to the renderer
		if err := s.processor.Render(w, item, fileIdx); err != nil {
			return err
		}
		if _, ok := item.(InputFile); ok {
			fileIdx++
		}
	}

	return s.processor.engine.Footer.Execute(w, FooterContext{Global: s.processor.global})
}

// Prepare calculates totals and updates the internal global context.
func (s *Stream) Prepare() GlobalContext {
	var totalSize int64
	var totalTokens int64
	fileCount := 0
	sectionCount := 0

	for _, item := range s.items {
		switch v := item.(type) {
		case InputFile:
			totalSize += v.Size
			totalTokens += s.tokenCounter(v.Size, nil)
			fileCount++
		case SectionContext:
			sectionCount++
		}
	}

	s.processor.global.TotalFiles = fileCount
	s.processor.global.TotalSize = totalSize
	s.processor.global.TotalSections = sectionCount
	s.processor.global.TokenEstimate = totalTokens

	// Update items with the calculated global context
	for i, item := range s.items {
		switch v := item.(type) {
		case SectionContext:
			v.Global = s.processor.global
			v.Section = i + 1
			s.items[i] = v
		case PromptContext:
			v.Global = s.processor.global
			v.Section = i + 1
			s.items[i] = v
		}
	}

	return s.processor.global
}

func (s *Stream) GetEngine() *TemplateEngine {
	return s.processor.engine
}

func (s *Stream) GetGlobalContext() GlobalContext {
	return s.processor.global
}
