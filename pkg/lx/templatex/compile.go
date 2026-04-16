package templatex

import (
	"fmt"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
)

// Compile parses configuration templates into an engine.
func Compile(cfg *core.Config) (*core.TemplateEngine, error) {
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

	return &core.TemplateEngine{
		FileContent:   tContent,
		FileError:     tError,
		FileBinary:    tBinary,
		FileCompact:   tCompact,
		Section:       tSection,
		Prompt:        tPrompt,
		SectionHeader: tSecHeader,
		SectionFooter: tSecFooter,
		OutputHeader:  tHeader,
		OutputFooter:  tFooter,
		Stats:         tStats,
	}, nil
}
