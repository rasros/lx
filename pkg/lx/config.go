package lx

import (
	"fmt"
	"text/template"
	"time"

	"github.com/monochromegane/go-gitignore"
)

// TokenCounter allows plugging in different LLM tokenizers.
type TokenCounter func(size int64, content interface{}) int64

// DefaultTokenCounter provides a simple 4-char-per-token heuristic.
func DefaultTokenCounter(size int64, content interface{}) int64 {
	var targetSize int64 = size
	switch v := content.(type) {
	case string:
		targetSize = int64(len(v))
	case []byte:
		targetSize = int64(len(v))
	case fmt.Stringer:
		targetSize = int64(len(v.String()))
	}
	return targetSize / 4
}

// GlobalContext holds metadata about the entire execution.
type GlobalContext struct {
	TotalFiles        int
	TotalSize         int64
	TotalWrittenBytes int64
	TokenEstimate     int64
	TotalSections     int
	WorkDir           string
	Metadata          map[string]string
}

// RunnerConfig defines slicing and formatting state for the rendering processor.
type RunnerConfig struct {
	Head        int
	Tail        int
	LineNumbers bool
}

// TemplateEngine holds the parsed text/template instances.
type TemplateEngine struct {
	Main          *template.Template
	Section       *template.Template
	Prompt        *template.Template
	Stats         *template.Template
	Header        *template.Template
	Footer        *template.Template
	SectionHeader *template.Template
	SectionFooter *template.Template
}

// Config represents the core library configuration.
type Config struct {
	Template              string
	SectionTemplate       string
	PromptTemplate        string
	StatsTemplate         string
	HeaderTemplate        string
	FooterTemplate        string
	SectionHeaderTemplate string
	SectionFooterTemplate string
	OutputFormat          string
	FollowSymlinks        bool
	ShowHidden            bool
	IgnoreEnabled         bool
	GlobalIgnore          gitignore.IgnoreMatcher
}

// NewConfig returns a default configuration.
func NewConfig() *Config {
	return &Config{
		OutputFormat:  "markdown",
		IgnoreEnabled: true,
	}
}

func CompileTemplates(cfg *Config) (*TemplateEngine, error) {
	format := cfg.OutputFormat
	if format == "" {
		format = "markdown"
	}

	var defMain, defSection, defPrompt string
	defHeader := defaultHeaderTemplate
	defFooter := defaultFooterTemplate
	defSecHeader := ""
	defSecFooter := ""

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
	tSecHeader, err := parse("section_header", pick(cfg.SectionHeaderTemplate, defSecHeader))
	if err != nil {
		return nil, fmt.Errorf("section_header template: %w", err)
	}
	tSecFooter, err := parse("section_footer", pick(cfg.SectionFooterTemplate, defSecFooter))
	if err != nil {
		return nil, fmt.Errorf("section_footer template: %w", err)
	}

	return &TemplateEngine{
		Main:          tMain,
		Section:       tSection,
		Prompt:        tPrompt,
		Stats:         tStats,
		Header:        tHeader,
		Footer:        tFooter,
		SectionHeader: tSecHeader,
		SectionFooter: tSecFooter,
	}, nil
}

// Merge applies non-zero fields from src to dst.
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
	if src.SectionHeaderTemplate != "" {
		dst.SectionHeaderTemplate = src.SectionHeaderTemplate
	}
	if src.SectionFooterTemplate != "" {
		dst.SectionFooterTemplate = src.SectionFooterTemplate
	}
	if src.OutputFormat != "" {
		dst.OutputFormat = src.OutputFormat
	}
	if src.FollowSymlinks {
		dst.FollowSymlinks = true
	}
	if src.ShowHidden {
		dst.ShowHidden = true
	}
}

// --- Context Structs for Templates ---

type FileContext struct {
	Path             string
	AbsPath          string
	Size             int64
	ModTime          time.Time
	TotalRows        int
	TokenEstimate    int64
	IsEstimate       bool
	Language         string
	Content          interface{}
	IsBinary         bool
	IsImage          bool
	IsCompactView    bool
	FileIndex        int
	SectionFileIndex int
	Separator        string
	Global           GlobalContext
	Section          SectionContext
}

type SectionContext struct {
	Body       string
	Index      int
	TotalFiles int
	TotalSize  int64
	Separator  string
	Global     GlobalContext
	IsImplicit bool
}

type PromptContext struct {
	Body      string
	Separator string
	Global    GlobalContext
	Section   SectionContext
}

type HeaderContext struct {
	Global GlobalContext
}

type FooterContext struct {
	Global GlobalContext
}

type StatsContext struct {
	Global GlobalContext
}
