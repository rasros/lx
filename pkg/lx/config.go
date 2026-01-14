package lx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Config represents the file-based configuration.
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
	Ignore         *bool `yaml:"ignore"` // Pointer to distinguish between unset (nil) and false

	// Runtime only
	Logger        *Logger  `yaml:"-"`
	LogLevel      LogLevel `yaml:"-"`
	LoadedConfigs []string `yaml:"-"`
}

// RunnerConfig is the runtime configuration for a specific file/operation.
type RunnerConfig struct {
	Head        int
	Tail        int
	LineNumbers bool
}

type TemplateEngine struct {
	Main    *template.Template
	Section *template.Template
	Prompt  *template.Template
	Stats   *template.Template
	Header  *template.Template
	Footer  *template.Template
}

// Options represents CLI flags that override Config.
type Options struct {
	Head  int
	Tail  int
	NBoth int

	HeadSet bool
	TailSet bool
	NSet    bool

	ConfigPath   string
	LineNumbers  bool
	OutputFormat string

	// Filter state
	Includes []string
	Excludes []string
}

// ToRunnerConfig resolves the specific head/tail counts based on CLI flags.
func (o Options) ToRunnerConfig() RunnerConfig {
	head, tail := -1, -1 // -1 indicates "read all"

	// -n/--lines splits budget between head and tail.
	if o.NSet {
		total := o.NBoth
		if total < 0 {
			total = 0
		}

		switch {
		case o.HeadSet:
			head = clamp(o.Head, 0, total)
			tail = total - head

		case o.TailSet:
			tail = clamp(o.Tail, 0, total)
			head = total - tail

		// Split evenly, favoring Head.
		default:
			head = (total + 1) / 2
			tail = total / 2
		}
	} else {
		if o.HeadSet {
			head = o.Head
			if !o.TailSet {
				tail = 0
			}
		}
		if o.TailSet {
			tail = o.Tail
			if !o.HeadSet {
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
	defHeader := DefaultHeaderTemplate
	defFooter := DefaultFooterTemplate

	switch format {
	case "xml":
		defMain = DefaultXMLTemplate
		defSection = DefaultXMLSectionTemplate
		defPrompt = DefaultXMLPromptTemplate
	case "html":
		defMain = DefaultHTMLTemplate
		defSection = DefaultHTMLSectionTemplate
		defPrompt = DefaultHTMLPromptTemplate
		defHeader = DefaultHTMLHeaderTemplate
		defFooter = DefaultHTMLFooterTemplate
	default:
		defMain = DefaultTemplate
		defSection = DefaultSectionTemplate
		defPrompt = DefaultPromptTemplate
	}

	pick := func(user, def string) string {
		if user != "" {
			return user
		}
		return def
	}

	funcs := TemplateFuncs()
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

	tStats, err := parse("stats", pick(cfg.StatsTemplate, DefaultStatsTemplate))
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

func (c *Config) ApplyGlobals(globals map[string]string) {
	if _, ok := globals["follow"]; ok {
		c.FollowSymlinks = true
	} else if _, ok := globals["no-follow"]; ok {
		c.FollowSymlinks = false
	}

	if _, ok := globals["hidden"]; ok {
		c.ShowHidden = true
	} else if _, ok := globals["no-hidden"]; ok {
		c.ShowHidden = false
	}

	if _, ok := globals["ignore"]; ok {
		t := true
		c.Ignore = &t
	} else if _, ok := globals["no-ignore"]; ok {
		f := false
		c.Ignore = &f
	}

	if c.Verbosity != "" {
		c.LogLevel = ParseLevel(c.Verbosity)
	} else {
		c.LogLevel = LevelWarn
	}

	if _, ok := globals["quiet"]; ok {
		c.LogLevel = LevelSilent
	} else if v, ok := globals["verbose"]; ok {
		count, err := strconv.Atoi(v)
		if err == nil {
			if count >= 3 {
				c.LogLevel = LevelTrace
			} else if count == 2 {
				c.LogLevel = LevelDebug
			} else if count == 1 {
				c.LogLevel = LevelInfo
			}
		} else {
			// Fallback if somehow a string got in (e.g. from future config logic merging)
			c.LogLevel = ParseLevel(v)
		}
	}

	if _, ok := globals["stats"]; ok {
		c.ShowStats = "always"
	} else if _, ok := globals["no-stats"]; ok {
		c.ShowStats = "never"
	} else if c.LogLevel == LevelSilent {
		// Quiet suppresses stats too unless forced
		c.ShowStats = "never"
	}
}

// IgnoreEnabled returns the effective boolean value for Ignore.
// Defaults to true if nil.
func (c *Config) IgnoreEnabled() bool {
	if c.Ignore == nil {
		return true
	}
	return *c.Ignore
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
