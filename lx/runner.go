package lx

import (
	"fmt"
	"io"
	"os"
	"text/template"
)

type Runner struct {
	Head            int
	Tail            int
	Template        *template.Template
	SectionTemplate *template.Template
	LineNumbers     bool
}

func NewRunner(head, tail int, tmpl, sectionTmpl *template.Template, lineNumbers bool) *Runner {
	return &Runner{
		Head:            head,
		Tail:            tail,
		Template:        tmpl,
		SectionTemplate: sectionTmpl,
		LineNumbers:     lineNumbers,
	}
}

// RunFile processes a single file with the current runner configuration.
// It is exported to support interleaved argument parsing.
func (r *Runner) RunFile(path string, index, total int, out io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}

	var view []byte
	var totalRows int
	var language string

	isBin := IsBinaryData(data)

	if !isBin {
		view, totalRows = prepareView(data, r.Head, r.Tail)
		language = DetectLanguage(path, data)
		if r.LineNumbers {
			view = addLineNumbers(view, totalRows, r.Head, r.Tail)
		}
	}

	ctx := FileContext{
		Path:       path,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		TotalRows:  totalRows,
		Language:   language,
		Content:    string(view),
		IsBinary:   isBin,
		FileIndex:  index,
		TotalFiles: total,
	}

	if err := r.Template.Execute(out, ctx); err != nil {
		return fmt.Errorf("template exec: %w", err)
	}

	return nil
}

// RunSection processes a section header with the current runner configuration.
func (r *Runner) RunSection(name string, out io.Writer) error {
	ctx := SectionContext{
		Name: name,
	}
	if err := r.SectionTemplate.Execute(out, ctx); err != nil {
		return fmt.Errorf("section template exec: %w", err)
	}
	return nil
}

// Run processes a list of files using the current runner configuration.
func (r *Runner) Run(files []string, out io.Writer) error {
	total := len(files)
	for i, path := range files {
		if err := r.RunFile(path, i+1, total, out); err != nil {
			return fmt.Errorf("lx: %w", err)
		}
	}
	return nil
}
