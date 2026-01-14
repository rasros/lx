package lx

import (
	"fmt"
	"text/template"
	"time"

	"github.com/monochromegane/go-gitignore"
)

// GlobalContext holds metadata about the entire execution across all files.
type GlobalContext struct {
	TotalFiles    int
	TotalSize     int64
	TokenEstimate int64
	TotalSections int
	WorkDir       string
	Metadata      map[string]string
	Config        Config
}

// RunnerConfig defines slicing and formatting state for the rendering processor.
type RunnerConfig struct {
	Head        int
	Tail        int
	LineNumbers bool
}

// TemplateEngine holds the parsed text/template instances for all output modes.
type TemplateEngine struct {
	Main    *template.Template
	Section *template.Template
	Prompt  *template.Template
	Stats   *template.Template
	Header  *template.Template
	Footer  *template.Template
}

// Config represents the configuration for the lx engine.
type Config struct {
	Template        string `yaml:"template"`
	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
	StatsTemplate   string `yaml:"stats_template"`
	HeaderTemplate  string `yaml:"header_template"`
	FooterTemplate  string `yaml:"footer_template"`

	OutputMode   string `yaml:"output_mode"`
	OutputFormat string `yaml:"output_format"`

	ShowStats string `yaml:"show_stats"`
	Verbosity string `yaml:"verbosity"`

	FollowSymlinks bool  `yaml:"follow_symlinks"`
	ShowHidden     bool  `yaml:"show_hidden"`
	Ignore         *bool `yaml:"ignore"`

	// GlobalIgnore is an optional matcher for global excludes.
	GlobalIgnore gitignore.IgnoreMatcher `yaml:"-"`

	// LoadedConfigs tracks which config files were successfully loaded.
	LoadedConfigs []string `yaml:"-"`
}

// NewConfig initializes a default configuration with safe defaults.
func NewConfig() *Config {
	ignore := true
	return &Config{
		OutputFormat: "markdown",
		OutputMode:   "stdout",
		Ignore:       &ignore,
		ShowStats:    "auto",
		Verbosity:    "warn",
	}
}

// IgnoreEnabled returns the effective boolean value for the Ignore pointer.
func (c *Config) IgnoreEnabled() bool {
	if c.Ignore == nil {
		return true
	}
	return *c.Ignore
}

// CompileTemplates compiles the templates defined in the Config.
// It applies defaults for any missing templates based on the OutputFormat.
func CompileTemplates(cfg *Config) (*TemplateEngine, error) {
	format := cfg.OutputFormat
	if format == "" {
		format = "markdown"
	}

	var defMain, defSection, defPrompt string
	defHeader := defaultHeaderTemplate
	defFooter := defaultFooterTemplate

	switch format {
	case "xml":
		defMain = defaultXMLTemplate
		defSection = defaultXMLSectionTemplate
		defPrompt = defaultXMLPromptTemplate
	case "html":
		defMain = defaultHTMLTemplate
		defSection = defaultHTMLSectionTemplate
		defPrompt = defaultHTMLPromptTemplate
		defHeader = defaultHTMLHeaderTemplate
		defFooter = defaultHTMLFooterTemplate
	default:
		defMain = defaultTemplate
		defSection = defaultSectionTemplate
		defPrompt = defaultPromptTemplate
	}

	pick := func(user, def string) string {
		if user != "" {
			return user
		}
		return def
	}

	funcs := templateFuncs()
	parse := func(name, tmpl string) (*template.Template, error) {
		return template.New(name).Funcs(funcs).Parse(tmpl)
	}

	tMain, err := parse("main", pick(cfg.Template, defMain))
	if err != nil {
		return nil, fmt.Errorf("main template: %w", err)
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, defSection))
	if err != nil {
		return nil, fmt.Errorf("section template: %w", err)
	}

	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, defPrompt))
	if err != nil {
		return nil, fmt.Errorf("prompt template: %w", err)
	}

	tStats, err := parse("stats", pick(cfg.StatsTemplate, defaultStatsTemplate))
	if err != nil {
		return nil, fmt.Errorf("stats template: %w", err)
	}

	tHeader, err := parse("header", pick(cfg.HeaderTemplate, defHeader))
	if err != nil {
		return nil, fmt.Errorf("header template: %w", err)
	}

	tFooter, err := parse("footer", pick(cfg.FooterTemplate, defFooter))
	if err != nil {
		return nil, fmt.Errorf("footer template: %w", err)
	}

	return &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
		Stats:   tStats,
		Header:  tHeader,
		Footer:  tFooter,
	}, nil
}

// Merge overrides values in dst with non-zero values from src.
func Merge(dst *Config, src *Config) {
	if src.Template != "" {
		dst.Template = src.Template
	}
	if src.SectionTemplate != "" {
		dst.SectionTemplate = src.SectionTemplate
	}
	if src.PromptTemplate != "" {
		dst.PromptTemplate = src.PromptTemplate
	}
	if src.StatsTemplate != "" {
		dst.StatsTemplate = src.StatsTemplate
	}
	if src.HeaderTemplate != "" {
		dst.HeaderTemplate = src.HeaderTemplate
	}
	if src.FooterTemplate != "" {
		dst.FooterTemplate = src.FooterTemplate
	}
	if src.OutputMode != "" {
		dst.OutputMode = src.OutputMode
	}
	if src.OutputFormat != "" {
		dst.OutputFormat = src.OutputFormat
	}
	if src.ShowStats != "" {
		dst.ShowStats = src.ShowStats
	}
	if src.Verbosity != "" {
		dst.Verbosity = src.Verbosity
	}
	if src.Ignore != nil {
		dst.Ignore = src.Ignore
	}
	if src.FollowSymlinks {
		dst.FollowSymlinks = true
	}
	if src.ShowHidden {
		dst.ShowHidden = true
	}
}

// FileContext is passed to the main template during file rendering.
type FileContext struct {
	Path           string
	AbsPath        string
	Size           int64
	ModTime        time.Time
	TotalRows      int
	TokenEstimate  int64
	IsEstimate     bool
	Language       string
	Content        interface{}
	IsBinary       bool
	IsImage        bool
	IsCompactView  bool
	FileIndex      int
	CurrentSection int
	Global         GlobalContext
}

// SectionContext is passed to the section template.
type SectionContext struct {
	Body    string
	Section int
	Global  GlobalContext
}

// PromptContext is passed to the prompt template.
type PromptContext struct {
	Body    string
	Section int
	Global  GlobalContext
}

// HeaderContext is passed to the header template.
type HeaderContext struct {
	Global GlobalContext
}

// FooterContext is passed to the footer template.
type FooterContext struct {
	Global GlobalContext
}

// StatsContext is passed to the stats template.
type StatsContext struct {
	Global GlobalContext
}
