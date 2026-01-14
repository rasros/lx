package lx

import (
	"fmt"
	"io"
	"log/slog"
	"text/template"

	"github.com/monochromegane/go-gitignore"
)

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

	// Logger handles debug and info output. If nil, a no-op logger is used.
	Logger *slog.Logger `yaml:"-"`

	// GlobalIgnore is an optional matcher for global excludes (e.g. from ~/.config/git/ignore).
	GlobalIgnore gitignore.IgnoreMatcher `yaml:"-"`

	// LoadedConfigs tracks which config files were successfully loaded (for debug info).
	LoadedConfigs []string `yaml:"-"`
}

// EnsureLogger guarantees that c.Logger is non-nil.
func (c *Config) EnsureLogger() {
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

// IgnoreEnabled returns the effective boolean value for Ignore.
func (c *Config) IgnoreEnabled() bool {
	if c.Ignore == nil {
		return true
	}
	return *c.Ignore
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

	// Booleans and pointers are only overridden if meaningful logic applies.
	// For simplicty in this specific app, we usually load base config then apply flags.
	// CLI flags are handled via ApplyOptions usually, but if merging two config files:
	if src.Ignore != nil {
		dst.Ignore = src.Ignore
	}
	// Note: basic bools (FollowSymlinks) are false by default, so difficult to distinguish
	// "unset" from "false" without pointers. For config files, last one wins if we parsed strictly,
	// but here we assume 'dst' is the accumulator.
	if src.FollowSymlinks {
		dst.FollowSymlinks = true
	}
	if src.ShowHidden {
		dst.ShowHidden = true
	}
}

// RunnerConfig is the runtime configuration for a specific file operation.
type RunnerConfig struct {
	Head        int
	Tail        int
	LineNumbers bool
}

// TemplateEngine holds the parsed text/template instances.
type TemplateEngine struct {
	Main    *template.Template
	Section *template.Template
	Prompt  *template.Template
	Stats   *template.Template
	Header  *template.Template
	Footer  *template.Template
}

// Options represents overrides usually provided by command-line flags.
type Options struct {
	Head  *int
	Tail  *int
	NBoth *int

	LineNumbers  bool
	OutputFormat string

	Includes []string
	Excludes []string
}

// ToRunnerConfig resolves the specific head/tail counts based on flags.
func (o Options) ToRunnerConfig() RunnerConfig {
	head, tail := -1, -1 // -1 indicates "read all"

	if o.NBoth != nil {
		total := *o.NBoth
		if total < 0 {
			total = 0
		}

		switch {
		case o.Head != nil:
			head = clamp(*o.Head, 0, total)
			tail = total - head
		case o.Tail != nil:
			tail = clamp(*o.Tail, 0, total)
			head = total - tail
		default:
			head = (total + 1) / 2
			tail = total / 2
		}
	} else {
		if o.Head != nil {
			head = *o.Head
			if o.Tail == nil {
				tail = 0
			}
		}
		if o.Tail != nil {
			tail = *o.Tail
			if o.Head == nil {
				head = 0
			}
		}
	}

	return RunnerConfig{
		Head:        head,
		Tail:        tail,
		LineNumbers: o.LineNumbers,
	}
}

// CompileTemplates compiles the templates defined in the Config.
// It applies defaults for any missing templates based on the OutputFormat.
func CompileTemplates(cfg *Config) (*TemplateEngine, error) {
	if cfg.OutputMode == "" {
		cfg.OutputMode = "stdout"
	}

	format := cfg.OutputFormat
	if format == "" {
		format = "markdown"
	}
	cfg.OutputFormat = format

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

	tMain, err := parse("lx", pick(cfg.Template, defMain))
	if err != nil {
		return nil, fmt.Errorf("parse main template: %w", err)
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, defSection))
	if err != nil {
		return nil, fmt.Errorf("parse section template: %w", err)
	}

	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, defPrompt))
	if err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}

	tStats, err := parse("stats", pick(cfg.StatsTemplate, defaultStatsTemplate))
	if err != nil {
		return nil, fmt.Errorf("parse stats template: %w", err)
	}

	tHeader, err := parse("header", pick(cfg.HeaderTemplate, defHeader))
	if err != nil {
		return nil, fmt.Errorf("parse header template: %w", err)
	}

	tFooter, err := parse("footer", pick(cfg.FooterTemplate, defFooter))
	if err != nil {
		return nil, fmt.Errorf("parse footer template: %w", err)
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

// ApplyOptions updates the config with values from Options (CLI flags).
func ApplyOptions(cfg *Config, opts Options) {
	if opts.OutputFormat != "" {
		cfg.OutputFormat = opts.OutputFormat
	}
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
