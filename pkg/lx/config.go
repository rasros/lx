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
	DebugTemplate   string `yaml:"debug_template"`

	OutputMode string `yaml:"output_mode"` // "stdout" (default) or "copy"
	DebugMode  string `yaml:"debug_mode"`  // "auto", "always", "never"

	FollowSymlinks bool  `yaml:"follow_symlinks"`
	ShowHidden     bool  `yaml:"show_hidden"`
	Ignore         *bool `yaml:"ignore"` // Pointer to distinguish between unset (nil) and false
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
	Debug   *template.Template
}

// Options represents CLI flags that override Config.
type Options struct {
	Head  int
	Tail  int
	NBoth int

	HeadSet bool
	TailSet bool
	NSet    bool

	ConfigPath  string
	LineNumbers bool
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

	// Set defaults for strings if empty
	if cfg.OutputMode == "" {
		cfg.OutputMode = "stdout"
	}

	// Helper to pick template or default
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

	tMain, err := parse("lx", pick(cfg.Template, DefaultTemplate))
	if err != nil {
		return nil, nil, fmt.Errorf("parse main template: %w", err)
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, DefaultSectionTemplate))
	if err != nil {
		return nil, nil, fmt.Errorf("parse section template: %w", err)
	}

	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, DefaultPromptTemplate))
	if err != nil {
		return nil, nil, fmt.Errorf("parse prompt template: %w", err)
	}

	tDebug, err := parse("debug", pick(cfg.DebugTemplate, DefaultDebugTemplate))
	if err != nil {
		return nil, nil, fmt.Errorf("parse debug template: %w", err)
	}

	return &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
		Debug:   tDebug,
	}, cfg, nil
}

// loadConfigChain loads config from Default -> Env -> CLI.
func loadConfigChain(cliPath string) (*Config, error) {
	cfg := &Config{}

	// 1. Load Defaults (User Config Dir)
	if configDir, err := os.UserConfigDir(); err == nil {
		_ = mergeConfig(cfg, filepath.Join(configDir, "lx", "config.yaml"), false)
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

	if _, ok := globals["quiet"]; ok {
		c.DebugMode = "never"
	} else if _, ok := globals["verbose"]; ok {
		c.DebugMode = "always"
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
	return yaml.NewDecoder(f).Decode(cfg)
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
