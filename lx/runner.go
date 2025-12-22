package lx

import (
	"fmt"
	"io"
	"os"
)

type Runner struct {
	Config RunnerConfig
	Engine *TemplateEngine
}

func NewRunner(cfg RunnerConfig, engine *TemplateEngine) *Runner {
	return &Runner{
		Config: cfg,
		Engine: engine,
	}
}

func (r *Runner) RunSection(body string, out io.Writer) error {
	return r.Engine.Section.Execute(out, struct{ Body string }{Body: body})
}

func (r *Runner) RunPrompt(body string, out io.Writer) error {
	return r.Engine.Prompt.Execute(out, struct{ Body string }{Body: body})
}

// RunFile processes a single file.
// It returns true if the file was "compact" (empty, binary, or skipped by user), false if it was a text block.
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
	isExplicitCompact := r.Config.Head == 0 && r.Config.Tail == 0
	isCompact := isEmpty || isBin || isExplicitCompact

	if prevCompact && !isCompact {
		if _, err := out.Write([]byte("\n")); err != nil {
			return false, err
		}
	}

	if isExplicitCompact {
		totalRows = countLines(data)
	} else if !isCompact {
		if r.Config.Head < 0 && r.Config.Tail < 0 {
			view = data
			totalRows = countLines(data)
		} else {
			view, totalRows = prepareView(data, r.Config.Head, r.Config.Tail)
		}

		language = DetectLanguage(path, data)
		if r.Config.LineNumbers {
			view = addLineNumbers(view, totalRows, r.Config.Head, r.Config.Tail)
		}
	}

	ctx := FileContext{
		Path:          path,
		Size:          info.Size(),
		ModTime:       info.ModTime(),
		TotalRows:     totalRows,
		Language:      language,
		Content:       string(view),
		IsBinary:      isBin,
		IsCompactView: isCompact,
		FileIndex:     index,
		TotalFiles:    total,
	}

	if err := r.Engine.Main.Execute(out, ctx); err != nil {
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
