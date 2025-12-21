package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
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

// RunSection renders the section template and ensures it is followed by a blank row.
func (r *Runner) RunSection(name string, out io.Writer) error {
	ctx := SectionContext{Name: name}

	// Render to buffer first so we can inspect the trailing newlines
	var buf bytes.Buffer
	if err := r.SectionTemplate.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("section template exec: %w", err)
	}

	content := buf.String()
	final := ensureBlankRow(content)

	if _, err := io.WriteString(out, final); err != nil {
		return fmt.Errorf("write section: %w", err)
	}
	return nil
}

// RunPrompt writes text preceded by a newline (separation) and followed by a blank row.
func (r *Runner) RunPrompt(text string, out io.Writer) error {
	// Prepend newline for separation from previous output (like Section does)
	if !strings.HasPrefix(text, "\n") {
		text = "\n" + text
	}

	final := ensureBlankRow(text)
	if _, err := io.WriteString(out, final); err != nil {
		return fmt.Errorf("write prompt: %w", err)
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

// ensureBlankRow ensures the string ends with at least two newlines (\n\n).
func ensureBlankRow(s string) string {
	if strings.HasSuffix(s, "\n\n") {
		return s
	}
	if strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s + "\n\n"
}
