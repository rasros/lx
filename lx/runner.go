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

func (r *Runner) RunFile(path string, index, total int, prevCompact bool, out io.Writer) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %q: %w", path, err)
	}

	fileSize := info.Size()

	isExplicitCompact := r.Config.Head == 0 && r.Config.Tail == 0
	isUnlimited := r.Config.Head < 0 && r.Config.Tail < 0

	var (
		headBytes, tailBytes, gapBytes []byte
		totalRows                      int
		isEstimate                     bool
		language                       string
		isBin                          bool
	)

	if prevCompact && !isExplicitCompact {
		out.Write([]byte("\n"))
	}

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if isExplicitCompact {
		var exact bool
		totalRows, exact, _ = EstimateLineCount(f, fileSize)
		isEstimate = !exact
	} else {
		header := make([]byte, 1024)
		n, _ := f.ReadAt(header, 0)
		isBin = IsBinary(header[:n])

		if !isBin {
			if isUnlimited {
				f.Seek(0, 0)
				headBytes, totalRows, _ = ReadHead(f, -1)
			} else {
				var exact bool
				totalRows, exact, _ = EstimateLineCount(f, fileSize)
				isEstimate = !exact

				if r.Config.Head > 0 {
					f.Seek(0, 0)
					headBytes, _, _ = ReadHead(f, r.Config.Head)
				}

				if r.Config.Tail > 0 {
					if r.Config.Head > 0 {
						skipped := totalRows - r.Config.Head - r.Config.Tail
						if skipped < 0 {
							skipped = 0
						}

						tilde := ""
						if isEstimate {
							tilde = "~"
						}
						gapBytes = []byte(fmt.Sprintf("... (%s%d rows skipped)\n", tilde, skipped))
					}
					tailBytes, _ = ReadTailSeek(f, r.Config.Tail)
				}
			}

			if len(headBytes) > 0 {
				language = DetectLanguage(path, headBytes)
			} else if n > 0 {
				language = DetectLanguage(path, header[:n])
			}
		}
	}

	var content interface{}
	if r.Config.LineNumbers && !isBin && !isExplicitCompact {
		content = LineNumberFormatter{
			Head:      headBytes,
			Gap:       gapBytes,
			Tail:      tailBytes,
			TotalRows: totalRows,
		}
	} else if !isBin && !isExplicitCompact {
		if len(tailBytes) == 0 && len(gapBytes) == 0 {
			content = string(headBytes)
		} else {
			totalLen := len(headBytes) + len(gapBytes) + len(tailBytes)
			buf := make([]byte, 0, totalLen)
			buf = append(buf, headBytes...)
			buf = append(buf, gapBytes...)
			buf = append(buf, tailBytes...)
			content = string(buf)
		}
	}

	ctx := FileContext{
		Path:          path,
		Size:          fileSize,
		ModTime:       info.ModTime(),
		TotalRows:     totalRows,
		IsEstimate:    isEstimate,
		Language:      language,
		Content:       content,
		IsBinary:      isBin,
		IsCompactView: isExplicitCompact || isBin || fileSize == 0,
		FileIndex:     index,
		TotalFiles:    total,
	}

	if err := r.Engine.Main.Execute(out, ctx); err != nil {
		return false, fmt.Errorf("template exec: %w", err)
	}

	return ctx.IsCompactView, nil
}
