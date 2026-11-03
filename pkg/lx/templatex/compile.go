package templatex

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
)

// CheckFields renders t with data, a zero context of the kind t is given,
// and returns the error of a field the context does not have. It only finds
// fields on the path the zero context takes. Other errors are left to
// rendering, which real data may avoid.
func CheckFields(t *template.Template, data interface{}) error {
	err := t.Execute(io.Discard, data)
	var ee template.ExecError
	if errors.As(err, &ee) && strings.Contains(err.Error(), "can't evaluate field") {
		return err
	}
	return nil
}

// Compile parses configuration templates into an engine. A template that
// names a field its context lacks is an error too.
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

	engine := &core.TemplateEngine{}
	funcs := templateFuncs(engine)

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
	tTree, err := parse("tree", pick(cfg.TreeTemplate, defaults.Tree))
	if err != nil {
		return nil, fmt.Errorf("tree template: %w", err)
	}

	tMeta, err := parse("meta", pick(cfg.MetaTemplate, defaults.Meta))
	if err != nil {
		return nil, fmt.Errorf("meta template: %w", err)
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

	// A field a context lacks, such as one only the stats know, would fail
	// once output is under way; fail before any is written.
	for _, c := range []struct {
		name string
		t    *template.Template
		data interface{}
	}{
		{"file_content", tContent, &core.FileContext{}},
		{"file_error", tError, &core.FileContext{}},
		{"file_binary", tBinary, &core.FileContext{}},
		{"file_compact", tCompact, &core.FileContext{}},
		{"section", tSection, &core.SectionContext{}},
		{"prompt", tPrompt, &core.PromptContext{}},
		{"tree", tTree, &core.TreeContext{}},
		{"meta", tMeta, &core.MetaContext{}},
		{"section_header", tSecHeader, core.SectionContext{}},
		{"section_footer", tSecFooter, core.SectionContext{}},
		{"output_header", tHeader, core.HeaderContext{}},
		{"output_footer", tFooter, core.FooterContext{}},
	} {
		if err := CheckFields(c.t, c.data); err != nil {
			return nil, fmt.Errorf("%s template: %w", c.name, err)
		}
	}

	// HTML embeds images inline; every other format reports them like binaries.
	tImage := tBinary
	if format == "html" {
		tImage = tContent
	}

	*engine = core.TemplateEngine{
		FileContent:   tContent,
		FileError:     tError,
		FileBinary:    tBinary,
		FileCompact:   tCompact,
		FileImage:     tImage,
		Section:       tSection,
		Prompt:        tPrompt,
		Tree:          tTree,
		Meta:          tMeta,
		SectionHeader: tSecHeader,
		SectionFooter: tSecFooter,
		OutputHeader:  tHeader,
		OutputFooter:  tFooter,
		EmbedImages:   format == "html",
		UseSeparators: format != "html" && format != "bare",
	}
	return engine, nil
}
