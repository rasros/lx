package lx

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Config represents the file-based configuration.
type Config struct {
	Template        string `yaml:"template"`
	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
	StatsTemplate   string `yaml:"stats_template"`

	OutputMode   string `yaml:"output_mode"`   // "stdout" (default) or "copy"
	OutputFormat string `yaml:"output_format"` // "markdown" (default) or "xml"

	// Stats Control
	ShowStats string `yaml:"show_stats"` // "auto" (default), "always", "never"

	// Logging Verbosity (string in yaml)
	Verbosity string `yaml:"verbosity"`

	FollowSymlinks bool  `yaml:"follow_symlinks"`
	ShowHidden     bool  `yaml:"show_hidden"`
	Ignore         *bool `yaml:"ignore"` // Pointer to distinguish between unset (nil) and false

	// Runtime only
	Logger        *Logger  `yaml:"-"`
	LogLevel      LogLevel `yaml:"-"`
	LoadedConfigs []string `yaml:"-"` // Tracks loaded config files
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
	OutputFormat string // "markdown" or "xml" (overrides config)

	// Filter state
	Includes []string
	Excludes []string
}

// ToRunnerConfig resolves the specific head/tail counts based on CLI flags.
func (o Options) ToRunnerConfig() RunnerConfig {
	head, tail := -1, -1 // -1 indicates "read all"

	// 1. If -n/--lines is set, it acts as a total budget split between head and tail.
	if o.NSet {
		total := o.NBoth
		if total < 0 {
			total = 0
		}

		switch {
		// User specified -n and --head: Tail gets the remainder.
		case o.HeadSet:
			head = clamp(o.Head, 0, total)
			tail = total - head

		// User specified -n and --tail: Head gets the remainder.
		case o.TailSet:
			tail = clamp(o.Tail, 0, total)
			head = total - tail

		// User specified only -n: Split evenly (favoring Head if odd).
		default:
			head = (total + 1) / 2
			tail = total / 2
		}
	} else {
		// 2. Standard --head / --tail behavior (independent).
		if o.HeadSet {
			head = o.Head
			// If head is set but tail isn't, default tail to 0 (unless explicit)
			if !o.TailSet {
				tail = 0
			}
		}
		if o.TailSet {
			tail = o.Tail
			// If tail is set but head isn't, default head to 0
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
	switch format {
	case "xml":
		defMain = DefaultXMLTemplate
		defSection = DefaultXMLSectionTemplate
		defPrompt = DefaultXMLPromptTemplate
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

	return &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
		Stats:   tStats,
	}, cfg, nil
}

// loadConfigChain loads config from Default -> Env -> CLI.
func loadConfigChain(cliPath string) (*Config, error) {
	cfg := &Config{}

	// 1. Load Defaults (User Config Dir)
	if configDir, err := os.UserConfigDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = mergeConfig(cfg, path, false)
	}

	// 2. Load Env
	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		if err := mergeConfig(cfg, envPath, false); err != nil {
			return nil, fmt.Errorf("load env config: %w", err)
		}
	}

	// 3. Load CLI
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

	// Handle Ignore (pointer bool)
	if _, ok := globals["ignore"]; ok {
		t := true
		c.Ignore = &t
	} else if _, ok := globals["no-ignore"]; ok {
		f := false
		c.Ignore = &f
	}

	// Logging Level Logic
	// Default Logic: Start with Config value, fallback to Warn
	if c.Verbosity != "" {
		c.LogLevel = ParseLevel(c.Verbosity)
	} else {
		c.LogLevel = LevelWarn
	}

	// CLI Overrides
	if _, ok := globals["quiet"]; ok {
		c.LogLevel = LevelSilent
	} else if v, ok := globals["verbose"]; ok {
		c.LogLevel = ParseLevel(v)
	}

	// Stats Logic
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

	// Track successfully loaded config path
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
