package lx

import (
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Template string `yaml:"template"`
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
	effHead := o.Head
	effTail := o.Tail

	if o.NSet && o.NBoth > 0 {
		total := o.NBoth
		switch {
		case !o.HeadSet && !o.TailSet:
			effHead = (total + 1) / 2
			effTail = total / 2
		case o.HeadSet && !o.TailSet:
			h := o.Head
			if h < 0 {
				h = 0
			}
			if h > total {
				h = total
			}
			effHead = h
			effTail = total - h
		case !o.HeadSet && o.TailSet:
			t := o.Tail
			if t < 0 {
				t = 0
			}
			if t > total {
				t = total
			}
			effTail = t
			effHead = total - t
		}
	}

	tmplStr := DefaultTemplate
	if o.ConfigPath != "" {
		cfg, err := loadConfig(o.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		if cfg.Template != "" {
			tmplStr = cfg.Template
		}
	}

	t, err := template.New("lx").Funcs(TemplateFuncs()).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return NewRunner(
		effHead,
		effTail,
		t,
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
