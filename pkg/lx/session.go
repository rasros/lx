package lx

import (
	"fmt"
	"io"
	"log/slog"
)

// Session manages the state and lifecycle of an lx operation.
type Session struct {
	Config Config
	Logger *slog.Logger
	Engine *TemplateEngine
}

func NewSession(cfg *Config, logger *slog.Logger) (*Session, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	engine, err := CompileTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("compile templates: %w", err)
	}

	return &Session{
		Config: *cfg,
		Logger: logger,
		Engine: engine,
	}, nil
}

func (s *Session) NewWalker() *Walker {
	return NewWalker(s.Config, s.Logger)
}

func (s *Session) NewRunner(cfg RunnerConfig, global GlobalContext) *Runner {
	return NewRunner(cfg, s.Engine, global, s.Logger)
}

// CalculateGlobalContext creates the global context for template rendering.
func (s *Session) CalculateGlobalContext(files []InputFile, sections int, workDir string, metadata map[string]string) GlobalContext {
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	return GlobalContext{
		TotalFiles:    len(files),
		TotalSize:     totalSize,
		TokenEstimate: EstimateTokens(totalSize),
		TotalSections: sections,
		WorkDir:       workDir,
		Metadata:      metadata,
		Config:        s.Config,
	}
}

// EstimateTokens provides a unified way to estimate LLM tokens (4 chars per token).
func EstimateTokens(size int64) int64 {
	if size <= 0 {
		return 0
	}
	return size / 4
}
