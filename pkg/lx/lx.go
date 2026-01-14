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
	processor *Processor
	items     []StreamItem
}

func NewStream(cfg *Config, runnerCfg RunnerConfig) (*Stream, error) {
	engine, err := CompileTemplates(cfg)
	if err != nil {
		return nil, err
	}
	global := GlobalContext{WorkDir: ".", Metadata: make(map[string]string), Config: *cfg}
	return &Stream{processor: NewProcessor(engine, runnerCfg, global)}, nil
}

func (s *Stream) WithRunnerConfig(cfg RunnerConfig) *Stream {
	s.processor.cfg = cfg
	return s
}

func (s *Stream) AddFile(f InputFile) *Stream { s.items = append(s.items, f); return s }
func (s *Stream) AddSection(title string) *Stream {
	s.items = append(s.items, SectionContext{Body: title, Global: s.processor.global})
	return s
}
func (s *Stream) AddPrompt(text string) *Stream {
	s.items = append(s.items, PromptContext{Body: text, Global: s.processor.global})
	return s
}

func (s *Stream) Execute(ctx context.Context, w io.Writer) error {
	// Re-calculate all totals right before starting the stream
	_ = s.GetGlobalContext()

	if err := s.processor.engine.Header.Execute(w, HeaderContext{Global: s.processor.global}); err != nil {
		return err
	}

	fileIdx := 1
	for _, item := range s.items {
		if err := s.processor.Render(w, item, fileIdx); err != nil {
			return err
		}
		if _, ok := item.(InputFile); ok {
			fileIdx++
		}
	}

	return s.processor.engine.Footer.Execute(w, FooterContext{Global: s.processor.global})
}

func (s *Stream) GetItems() []StreamItem {
	return s.items
}

func (s *Stream) GetEngine() *TemplateEngine {
	return s.processor.engine
}

func (s *Stream) GetGlobalContext() GlobalContext {
	var totalSize int64
	fileCount := 0
	sectionCount := 0
	for _, item := range s.items {
		switch v := item.(type) {
		case InputFile:
			totalSize += v.Size
			fileCount++
		case SectionContext:
			sectionCount++
		}
	}

	s.processor.global.TotalFiles = fileCount
	s.processor.global.TotalSize = totalSize
	s.processor.global.TotalSections = sectionCount
	// Ensure TokenEstimate is actually set here
	s.processor.global.TokenEstimate = totalSize / 4

	return s.processor.global
}

func (s *Stream) UpdateStats(count int, size int64) {
	s.processor.global.TotalFiles = count
	s.processor.global.TotalSize = size
	s.processor.global.TokenEstimate = size / 4
}
