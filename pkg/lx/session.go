package lx

import (
	"fmt"
	"io"
	"log/slog"
)

// Session manages the state and lifecycle of an lx operation.
// It serves as the factory for Walkers and Runners and handles
// high-level context aggregation.
type Session struct {
	Config Config
	Logger *slog.Logger
	Engine *TemplateEngine
}

// NewSession creates a new Session with the provided config and logger.
// If logger is nil, a no-op logger is used.
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

// NewWalker creates a file system walker configured with this session's settings.
func (s *Session) NewWalker() *Walker {
	return NewWalker(s.Config, s.Logger)
}

// NewRunner creates a rendering runner for specific execution options.
func (s *Session) NewRunner(cfg RunnerConfig, global GlobalContext) *Runner {
	return NewRunner(cfg, s.Engine, global, s.Logger)
}

// CalculateGlobalContext creates the global context for template rendering
// based on the discovered files.
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
