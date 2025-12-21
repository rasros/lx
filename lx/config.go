package lx

import (
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Template        string `yaml:"template"`
	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
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

func (o Options) Effective() (*Runner, error) {
	// Default to -1 (Unlimited)
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

	tmplStr := DefaultTemplate
	sectionTmplStr := DefaultSectionTemplate
	promptTmplStr := DefaultPromptTemplate

	if o.ConfigPath != "" {
		cfg, err := loadConfig(o.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		if cfg.Template != "" {
			tmplStr = cfg.Template
		}
		if cfg.SectionTemplate != "" {
			sectionTmplStr = cfg.SectionTemplate
		}
		if cfg.PromptTemplate != "" {
			promptTmplStr = cfg.PromptTemplate
		}
	}

	funcs := TemplateFuncs()
	tMain, err := template.New("lx").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	tSection, err := template.New("section").Funcs(funcs).Parse(sectionTmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse section template: %w", err)
	}

	tPrompt, err := template.New("prompt").Funcs(funcs).Parse(promptTmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}

	return NewRunner(
		effHead,
		effTail,
		tMain,
		tSection,
		tPrompt,
		o.LineNumbers,
	), nil
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
