package lx

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

type RunnerConfig struct {
	Head        int
	Tail        int
	LineNumbers bool
}

type TemplateEngine struct {
	Main    *template.Template
	Section *template.Template
	Prompt  *template.Template
}

type Config struct {
	Template        string `yaml:"template"`
	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
	OutputMode      string `yaml:"output_mode"`
}

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

func (o Options) ToRunnerConfig() RunnerConfig {
	effHead, effTail := -1, -1

	if o.NSet {
		if o.NBoth == 0 {
			effHead, effTail = 0, 0
		} else {
			total := o.NBoth
			switch {
			case o.HeadSet:
				h := o.Head
				if h < 0 {
					h = 0
				} else if h > total {
					h = total
				}
				effHead, effTail = h, total-h
			case o.TailSet:
				t := o.Tail
				if t < 0 {
					t = 0
				} else if t > total {
					t = total
				}
				effTail, effHead = t, total-t
			default:
				effHead, effTail = (total+1)/2, total/2
			}
		}
	} else {
		if o.HeadSet {
			effHead = o.Head
			if !o.TailSet {
				effTail = 0
			}
		}
		if o.TailSet {
			effTail = o.Tail
			if !o.HeadSet {
				effHead = 0
			}
		}
	}

	return RunnerConfig{
		Head:        effHead,
		Tail:        effTail,
		LineNumbers: o.LineNumbers,
	}
}

func (o Options) CompileTemplates() (*TemplateEngine, *Config, error) {
	var cfg Config

	configDir, err := os.UserConfigDir()
	if err == nil {
		defPath := filepath.Join(configDir, "lx", "config.yaml")
		if err := mergeConfig(&cfg, defPath, false); err != nil {
			return nil, nil, fmt.Errorf("load default config: %w", err)
		}
	}

	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		if err := mergeConfig(&cfg, envPath, false); err != nil {
			return nil, nil, fmt.Errorf("load env config: %w", err)
		}
	}

	if o.ConfigPath != "" {
		if err := mergeConfig(&cfg, o.ConfigPath, true); err != nil {
			return nil, nil, fmt.Errorf("load cli config: %w", err)
		}
	}

	tmplStr := DefaultTemplate
	if cfg.Template != "" {
		tmplStr = cfg.Template
	}

	sectionTmplStr := DefaultSectionTemplate
	if cfg.SectionTemplate != "" {
		sectionTmplStr = cfg.SectionTemplate
	}

	promptTmplStr := DefaultPromptTemplate
	if cfg.PromptTemplate != "" {
		promptTmplStr = cfg.PromptTemplate
	}

	funcs := TemplateFuncs()
	tMain, err := template.New("lx").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse template: %w", err)
	}

	tSection, err := template.New("section").Funcs(funcs).Parse(sectionTmplStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse section template: %w", err)
	}

	tPrompt, err := template.New("prompt").Funcs(funcs).Parse(promptTmplStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse prompt template: %w", err)
	}

	return &TemplateEngine{
		Main:    tMain,
		Section: tSection,
		Prompt:  tPrompt,
	}, &cfg, nil
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
