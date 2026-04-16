package lx

import (
	"fmt"
	"text/template"
	"time"
)

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
	Head              int
	Tail              int
	LineNumbers       bool
	SkeletonFunctions bool
	SkeletonTypes     bool
}

// TemplateEngine holds the parsed text/template instances.
type TemplateEngine struct {
	FileContent *template.Template
	FileError   *template.Template
	FileBinary  *template.Template
	FileCompact *template.Template

	Section *template.Template
	Prompt  *template.Template

	SectionHeader *template.Template
	SectionFooter *template.Template

	OutputHeader *template.Template
	OutputFooter *template.Template
	Stats        *template.Template
}

// Config represents the core library configuration.
type Config struct {
	FileContentTemplate string
	FileErrorTemplate   string
	FileBinaryTemplate  string
	FileCompactTemplate string
	FileHeaderTemplate  string

	SectionTemplate string
	PromptTemplate  string

	SectionHeaderTemplate string
	SectionFooterTemplate string

	OutputHeaderTemplate string
	OutputFooterTemplate string
	StatsTemplate        string

	OutputFormat string

	IgnoreFileSymlinks bool
	IgnoreDirSymlinks  bool
	IgnoreHidden       bool
	IgnoreEnabled      bool
	ExpandArchives     bool
	ExtractDocuments   bool
}

// NewConfig returns a default configuration.
func NewConfig() *Config {
	return &Config{
		OutputFormat:       "markdown",
		IgnoreEnabled:      true,
		IgnoreHidden:       true,
		IgnoreFileSymlinks: false,
		IgnoreDirSymlinks:  true,
		ExtractDocuments:   true,
	}
}

// CompileTemplates parses the configuration templates into a TemplateEngine.
func CompileTemplates(cfg *Config) (*TemplateEngine, error) {
	format := cfg.OutputFormat
	if format == "" {
		format = "markdown"
	}

	defaults := getFormatDefaults(format)

	pick := func(user, def string) string {
		if user != "" {
			return user
		}
		return def
	}

	funcs := templateFuncs()

	fileHeaderStr := pick(cfg.FileHeaderTemplate, defaults.FileHeader)

	parseWithPartial := func(name, tmpl string) (*template.Template, error) {
		t := template.New(name).Funcs(funcs)
		if _, err := t.Parse(tmpl); err != nil {
			return nil, err
		}
		if _, err := t.New("file_header").Parse(fileHeaderStr); err != nil {
			return nil, err
		}
		return t, nil
	}

	parse := func(name, tmpl string) (*template.Template, error) {
		return template.New(name).Funcs(funcs).Parse(tmpl)
	}

	tContent, err := parseWithPartial("file_content", pick(cfg.FileContentTemplate, defaults.FileContent))
	if err != nil {
		return nil, fmt.Errorf("file_content template: %w", err)
	}
	tError, err := parseWithPartial("file_error", pick(cfg.FileErrorTemplate, defaults.FileError))
	if err != nil {
		return nil, fmt.Errorf("file_error template: %w", err)
	}
	tBinary, err := parseWithPartial("file_binary", pick(cfg.FileBinaryTemplate, defaults.FileBinary))
	if err != nil {
		return nil, fmt.Errorf("file_binary template: %w", err)
	}
	tCompact, err := parseWithPartial("file_compact", pick(cfg.FileCompactTemplate, defaults.FileCompact))
	if err != nil {
		return nil, fmt.Errorf("file_compact template: %w", err)
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, defaults.Section))
	if err != nil {
		return nil, fmt.Errorf("section template: %w", err)
	}
	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, defaults.Prompt))
	if err != nil {
		return nil, fmt.Errorf("prompt template: %w", err)
	}

	tSecHeader, err := parse("section_header", pick(cfg.SectionHeaderTemplate, defaults.SectionHeader))
	if err != nil {
		return nil, fmt.Errorf("section_header template: %w", err)
	}
	tSecFooter, err := parse("section_footer", pick(cfg.SectionFooterTemplate, defaults.SectionFooter))
	if err != nil {
		return nil, fmt.Errorf("section_footer template: %w", err)
	}

	tHeader, err := parse("output_header", pick(cfg.OutputHeaderTemplate, defaults.OutputHeader))
	if err != nil {
		return nil, fmt.Errorf("output_header template: %w", err)
	}
	tFooter, err := parse("output_footer", pick(cfg.OutputFooterTemplate, defaults.OutputFooter))
	if err != nil {
		return nil, fmt.Errorf("output_footer template: %w", err)
	}
	tStats, err := parse("stats", pick(cfg.StatsTemplate, defaultStatsTemplate))
	if err != nil {
		return nil, fmt.Errorf("stats template: %w", err)
	}

	return &TemplateEngine{
		FileContent: tContent,
		FileError:   tError,
		FileBinary:  tBinary,
		FileCompact: tCompact,

		Section: tSection,
		Prompt:  tPrompt,

		SectionHeader: tSecHeader,
		SectionFooter: tSecFooter,

		OutputHeader: tHeader,
		OutputFooter: tFooter,
		Stats:        tStats,
	}, nil
}

// Merge applies non-zero fields from src to dst.
func Merge(dst *Config, src *Config) {
	if src.FileContentTemplate != "" {
		dst.FileContentTemplate = src.FileContentTemplate
	}
	if src.FileErrorTemplate != "" {
		dst.FileErrorTemplate = src.FileErrorTemplate
	}
	if src.FileBinaryTemplate != "" {
		dst.FileBinaryTemplate = src.FileBinaryTemplate
	}
	if src.FileCompactTemplate != "" {
		dst.FileCompactTemplate = src.FileCompactTemplate
	}
	if src.FileHeaderTemplate != "" {
		dst.FileHeaderTemplate = src.FileHeaderTemplate
	}

	if src.SectionTemplate != "" {
		dst.SectionTemplate = src.SectionTemplate
	}
	if src.PromptTemplate != "" {
		dst.PromptTemplate = src.PromptTemplate
	}

	if src.SectionHeaderTemplate != "" {
		dst.SectionHeaderTemplate = src.SectionHeaderTemplate
	}
	if src.SectionFooterTemplate != "" {
		dst.SectionFooterTemplate = src.SectionFooterTemplate
	}

	if src.OutputHeaderTemplate != "" {
		dst.OutputHeaderTemplate = src.OutputHeaderTemplate
	}
	if src.OutputFooterTemplate != "" {
		dst.OutputFooterTemplate = src.OutputFooterTemplate
	}
	if src.StatsTemplate != "" {
		dst.StatsTemplate = src.StatsTemplate
	}

	if src.OutputFormat != "" {
		dst.OutputFormat = src.OutputFormat
	}
	if !src.IgnoreDirSymlinks {
		dst.IgnoreDirSymlinks = false
	}
	if src.IgnoreFileSymlinks {
		dst.IgnoreFileSymlinks = true
	}
	if !src.IgnoreHidden {
		dst.IgnoreHidden = false
	}
}

// FileContext represents the data provided to file-level templates.
type FileContext struct {
	Path             string
	AbsPath          string
	Size             int64
	ModTime          time.Time
	TotalRows        int
	ReadError        string
	IsError          bool
	TokenEstimate    int64
	IsEstimate       bool
	Language         string
	Content          interface{}
	IsBinary         bool
	IsImage          bool
	IsCompactView    bool
	FileIndex        int
	SectionFileIndex int
	Global           GlobalContext
	Section          SectionContext
	SkeletonMode     string
}

// SectionContext represents the data provided to section templates.
type SectionContext struct {
	Body       string
	Index      int
	TotalFiles int
	TotalSize  int64
	Global     GlobalContext
	IsImplicit bool
}

// PromptContext represents the data provided to custom text prompt templates.
type PromptContext struct {
	Body    string
	Global  GlobalContext
	Section SectionContext
}

// HeaderContext represents the data provided to the overall output header.
type HeaderContext struct {
	Global GlobalContext
}

// FooterContext represents the data provided to the overall output footer.
type FooterContext struct {
	Global GlobalContext
}

// StatsContext represents the data provided to the statistics summary template.
type StatsContext struct {
	Global GlobalContext
}
