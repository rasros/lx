package lx

import (
	"fmt"
)

// Session manages the template engine lifecycle.
type Session struct {
	Engine *TemplateEngine
}

// NewSession initializes the template engine with the provided configuration.
func NewSession(cfg *Config) (*Session, error) {
	engine, err := CompileTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("compile templates: %w", err)
	}

	return &Session{
		Engine: engine,
	}, nil
}

// NewRunner creates a rendering runner for a specific set of slicing options.
func (s *Session) NewRunner(cfg RunnerConfig, global GlobalContext) *Runner {
	return NewRunner(cfg, s.Engine, global)
}

// CreateGlobalContext creates the data structure used by templates for global metadata.
func CreateGlobalContext(files []InputFile, sections int, workDir string, metadata map[string]string) GlobalContext {
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	return GlobalContext{
		TotalFiles:    len(files),
		TotalSize:     totalSize,
		TotalSections: sections,
		WorkDir:       workDir,
		Metadata:      metadata,
	}
}
