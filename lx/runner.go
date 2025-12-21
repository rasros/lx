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
	PromptTemplate  *template.Template
	LineNumbers     bool
}

func NewRunner(head, tail int, tmpl, sectionTmpl, promptTmpl *template.Template, lineNumbers bool) *Runner {
	return &Runner{
		Head:            head,
		Tail:            tail,
		Template:        tmpl,
		SectionTemplate: sectionTmpl,
		PromptTemplate:  promptTmpl,
		LineNumbers:     lineNumbers,
	}
}

func (r *Runner) RunSection(body string, out io.Writer) error {
	return r.SectionTemplate.Execute(out, struct{ Body string }{Body: body})
}

func (r *Runner) RunPrompt(body string, out io.Writer) error {
	return r.PromptTemplate.Execute(out, struct{ Body string }{Body: body})
}

// RunFile processes a single file.
// It returns true if the file was "compact" (empty or binary), false if it was a text block.
func (r *Runner) RunFile(path string, index, total int, prevCompact bool, out io.Writer) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %q: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %q: %w", path, err)
	}

	var view []byte
	var totalRows int
	var language string

	isEmpty := len(data) == 0
	isBin := !isEmpty && IsBinaryData(data)
	isCompact := isEmpty || isBin

	// Logic: If the previous output was compact (list-like) and this one is a Block (text),
	// we must force a newline to separate them.
	if prevCompact && !isCompact {
		if _, err := out.Write([]byte("\n")); err != nil {
			return false, err
		}
	}

	if !isCompact {
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
		return false, fmt.Errorf("template exec: %w", err)
	}

	return isCompact, nil
}

// Run processes a list of files.
func (r *Runner) Run(files []string, out io.Writer) error {
	total := len(files)
	prevCompact := false
	for i, path := range files {
		var err error
		prevCompact, err = r.RunFile(path, i+1, total, prevCompact, out)
		if err != nil {
			return fmt.Errorf("lx: %w", err)
		}
	}
	return nil
}
