package lx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
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

	// LoadedConfigs tracks which config files were successfully loaded
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

	ConfigPath   string
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

// CompileTemplates loads configuration and parses all required templates.
func (o Options) CompileTemplates() (*TemplateEngine, *Config, error) {
	cfg, err := loadConfigChain(o.ConfigPath)
	if err != nil {
		return nil, nil, err
	}

	if cfg.OutputMode == "" {
		cfg.OutputMode = "stdout"
	}

	format := cfg.OutputFormat
	if o.OutputFormat != "" {
		format = o.OutputFormat
	}
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
		return nil, nil, fmt.Errorf("parse main template: %w", err)
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, defSection))
	if err != nil {
		return nil, nil, fmt.Errorf("parse section template: %w", err)
	}

	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, defPrompt))
	if err != nil {
		return nil, nil, fmt.Errorf("parse prompt template: %w", err)
	}

	tStats, err := parse("stats", pick(cfg.StatsTemplate, defaultStatsTemplate))
	if err != nil {
		return nil, nil, fmt.Errorf("parse stats template: %w", err)
	}

	tHeader, err := parse("header", pick(cfg.HeaderTemplate, defHeader))
	if err != nil {
		return nil, nil, fmt.Errorf("parse header template: %w", err)
	}

	tFooter, err := parse("footer", pick(cfg.FooterTemplate, defFooter))
	if err != nil {
		return nil, nil, fmt.Errorf("parse footer template: %w", err)
	}

	return &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
		Stats:   tStats,
		Header:  tHeader,
		Footer:  tFooter,
	}, cfg, nil
}

// loadConfigChain loads config from Default -> Env -> CLI.
func loadConfigChain(cliPath string) (*Config, error) {
	cfg := &Config{}

	if configDir, err := os.UserConfigDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = mergeConfig(cfg, path, false)
	}

	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		if err := mergeConfig(cfg, envPath, false); err != nil {
			return nil, fmt.Errorf("load env config: %w", err)
		}
	}

	if cliPath != "" {
		if err := mergeConfig(cfg, cliPath, true); err != nil {
			return nil, fmt.Errorf("load cli config: %w", err)
		}
	}

	return cfg, nil
}

func mergeConfig(cfg *Config, path string, strict bool) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		if strict {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return err
	}

	cfg.LoadedConfigs = append(cfg.LoadedConfigs, path)
	return nil
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
