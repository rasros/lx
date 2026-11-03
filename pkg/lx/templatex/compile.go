package templatex

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
)

// CheckFields renders t with data, a zero context, and returns the error of
// a field the context does not have. Other errors are left to rendering.
func CheckFields(t *template.Template, data interface{}) error {
	err := t.Execute(io.Discard, data)
	var ee template.ExecError
	if errors.As(err, &ee) && strings.Contains(err.Error(), "can't evaluate field") {
		return err
	}
	return nil
}

// Compile parses configuration templates into an engine, filling what cfg
// leaves unset from the format's profile. An unknown format or a template
// naming a missing field is an error.
func Compile(cfg *core.Config) (*core.TemplateEngine, error) {
	prof, err := profileFor(cfg.OutputFormat)
	if err != nil {
		return nil, err
	}

	pick := func(user, def string) string {
		if user != "" {
			return user
		}
		return def
	}
	pickBool := func(user *bool, def bool) bool {
		if user != nil {
			return *user
		}
		return def
	}

	engine := &core.TemplateEngine{}
	funcs := templateFuncs(engine)

	fileHeaderStr := pick(cfg.FileHeaderTemplate, prof.FileHeader)
	fileContentStr := pick(cfg.FileContentTemplate, prof.FileContent)
	fileBinaryStr := pick(cfg.FileBinaryTemplate, prof.FileBinary)

	// parseFile parses a file template with the partials it may call.
	parseFile := func(name, tmpl string, partials map[string]string) (*template.Template, error) {
		t := template.New(name).Funcs(funcs)
		if _, err := t.Parse(tmpl); err != nil {
			return nil, fmt.Errorf("%s template: %w", name, err)
		}
		for _, p := range []string{"file_header", "file_content", "file_binary"} {
			src, ok := partials[p]
			if !ok {
				continue
			}
			if _, err := t.New(p).Parse(src); err != nil {
				return nil, fmt.Errorf("%s template: %w", name, err)
			}
		}
		return t, nil
	}
	header := map[string]string{"file_header": fileHeaderStr}

	parse := func(name, tmpl string) (*template.Template, error) {
		return template.New(name).Funcs(funcs).Parse(tmpl)
	}

	tContent, err := parseFile("file_content", fileContentStr, header)
	if err != nil {
		return nil, err
	}
	tError, err := parseFile("file_error", pick(cfg.FileErrorTemplate, prof.FileError), header)
	if err != nil {
		return nil, err
	}
	tBinary, err := parseFile("file_binary", fileBinaryStr, header)
	if err != nil {
		return nil, err
	}
	tCompact, err := parseFile("file_compact", pick(cfg.FileCompactTemplate, prof.FileCompact), header)
	if err != nil {
		return nil, err
	}
	tImage, err := parseFile("file_image", pick(cfg.FileImageTemplate, prof.FileImage), map[string]string{
		"file_header":  fileHeaderStr,
		"file_content": fileContentStr,
		"file_binary":  fileBinaryStr,
	})
	if err != nil {
		return nil, err
	}

	tSection, err := parse("section", pick(cfg.SectionTemplate, prof.Section))
	if err != nil {
		return nil, fmt.Errorf("section template: %w", err)
	}
	tPrompt, err := parse("prompt", pick(cfg.PromptTemplate, prof.Prompt))
	if err != nil {
		return nil, fmt.Errorf("prompt template: %w", err)
	}
	tTree, err := parse("tree", pick(cfg.TreeTemplate, prof.Tree))
	if err != nil {
		return nil, fmt.Errorf("tree template: %w", err)
	}

	tMeta, err := parse("meta", pick(cfg.MetaTemplate, prof.Meta))
	if err != nil {
		return nil, fmt.Errorf("meta template: %w", err)
	}

	tSecHeader, err := parse("section_header", pick(cfg.SectionHeaderTemplate, prof.SectionHeader))
	if err != nil {
		return nil, fmt.Errorf("section_header template: %w", err)
	}
	tSecFooter, err := parse("section_footer", pick(cfg.SectionFooterTemplate, prof.SectionFooter))
	if err != nil {
		return nil, fmt.Errorf("section_footer template: %w", err)
	}

	tHeader, err := parse("output_header", pick(cfg.OutputHeaderTemplate, prof.OutputHeader))
	if err != nil {
		return nil, fmt.Errorf("output_header template: %w", err)
	}
	tFooter, err := parse("output_footer", pick(cfg.OutputFooterTemplate, prof.OutputFooter))
	if err != nil {
		return nil, fmt.Errorf("output_footer template: %w", err)
	}

	// Fail on a missing field before any output is written.
	for _, c := range []struct {
		name string
		t    *template.Template
		data interface{}
	}{
		{"file_content", tContent, &core.FileContext{}},
		{"file_error", tError, &core.FileContext{}},
		{"file_binary", tBinary, &core.FileContext{}},
		{"file_compact", tCompact, &core.FileContext{}},
		{"file_image", tImage, &core.FileContext{}},
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
		EmbedImages:   pickBool(cfg.EmbedImages, prof.EmbedImages),
		UseSeparators: pickBool(cfg.Separators, prof.Separators),
	}
	return engine, nil
}
