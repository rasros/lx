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

	engine := &core.TemplateEngine{
		EmbedImages:   pickBool(cfg.EmbedImages, prof.EmbedImages),
		UseSeparators: pickBool(cfg.Separators, prof.Separators),
	}
	funcs := templateFuncs(engine)

	// The partials a file template may call.
	partial := map[string]string{
		"file_header":  pick(cfg.FileHeaderTemplate, prof.FileHeader),
		"file_content": pick(cfg.FileContentTemplate, prof.FileContent),
		"file_binary":  pick(cfg.FileBinaryTemplate, prof.FileBinary),
	}
	header := []string{"file_header"}

	specs := []struct {
		name     string
		tmpl     string
		partials []string
		zero     interface{}
		dst      **template.Template
	}{
		{"file_content", partial["file_content"], header, &core.FileContext{}, &engine.FileContent},
		{"file_error", pick(cfg.FileErrorTemplate, prof.FileError), header, &core.FileContext{}, &engine.FileError},
		{"file_binary", partial["file_binary"], header, &core.FileContext{}, &engine.FileBinary},
		{"file_compact", pick(cfg.FileCompactTemplate, prof.FileCompact), header, &core.FileContext{}, &engine.FileCompact},
		{"file_image", pick(cfg.FileImageTemplate, prof.FileImage), []string{"file_header", "file_content", "file_binary"}, &core.FileContext{}, &engine.FileImage},
		{"section", pick(cfg.SectionTemplate, prof.Section), nil, &core.SectionContext{}, &engine.Section},
		{"prompt", pick(cfg.PromptTemplate, prof.Prompt), nil, &core.PromptContext{}, &engine.Prompt},
		{"tree", pick(cfg.TreeTemplate, prof.Tree), nil, &core.TreeContext{}, &engine.Tree},
		{"meta", pick(cfg.MetaTemplate, prof.Meta), nil, &core.MetaContext{}, &engine.Meta},
		{"section_header", pick(cfg.SectionHeaderTemplate, prof.SectionHeader), nil, core.SectionContext{}, &engine.SectionHeader},
		{"section_footer", pick(cfg.SectionFooterTemplate, prof.SectionFooter), nil, core.SectionContext{}, &engine.SectionFooter},
		{"output_header", pick(cfg.OutputHeaderTemplate, prof.OutputHeader), nil, core.HeaderContext{}, &engine.OutputHeader},
		{"output_footer", pick(cfg.OutputFooterTemplate, prof.OutputFooter), nil, core.FooterContext{}, &engine.OutputFooter},
	}

	for _, s := range specs {
		t, err := template.New(s.name).Funcs(funcs).Parse(s.tmpl)
		for _, p := range s.partials {
			if err == nil {
				_, err = t.New(p).Parse(partial[p])
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s template: %w", s.name, err)
		}
		*s.dst = t
	}
	// Fail on a missing field before any output is written.
	for _, s := range specs {
		if err := CheckFields(*s.dst, s.zero); err != nil {
			return nil, fmt.Errorf("%s template: %w", s.name, err)
		}
	}
	return engine, nil
}
