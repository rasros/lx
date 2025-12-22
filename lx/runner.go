package lx

import (
	"fmt"
	"io"
	"os"
)

// 10MB Threshold for switching strategies
const LargeFileThreshold = 10 * 1024 * 1024

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
	isLarge := fileSize > LargeFileThreshold
	isExplicitCompact := r.Config.Head == 0 && r.Config.Tail == 0

	// Use Seek Strategy only for Large Files with explicit limits
	hasLimits := r.Config.Head >= 0 || r.Config.Tail >= 0
	useSeekStrategy := isLarge && (hasLimits || isExplicitCompact)

	var view []byte
	var totalRows int
	var isEstimate bool
	var language string
	var isBin bool

	if prevCompact && !isExplicitCompact {
		out.Write([]byte("\n"))
	}

	if useSeekStrategy {
		// --- STRATEGY: LARGE FILE (Seek & Estimate) ---
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()

		if isExplicitCompact {
			totalRows, _ = EstimateLineCount(f, fileSize)
			isEstimate = true
		} else {
			header := make([]byte, 1024)
			n, _ := f.ReadAt(header, 0)
			isBin = IsBinary(header[:n])

			if !isBin {
				totalRows, _ = EstimateLineCount(f, fileSize)
				isEstimate = true

				if r.Config.Head > 0 {
					f.Seek(0, 0)
					headBytes, _ := ReadHead(f, r.Config.Head)
					view = append(view, headBytes...)
				}

				if r.Config.Tail > 0 {
					if r.Config.Head > 0 {
						skipped := totalRows - r.Config.Head - r.Config.Tail
						if skipped < 0 {
							skipped = 0
						}
						view = append(view, []byte(fmt.Sprintf("... (~%d rows skipped)\n", skipped))...)
					}
					tailBytes, _ := ReadTailSeek(f, r.Config.Tail)
					view = append(view, tailBytes...)
				}

				language = DetectLanguage(path, header[:n])
			}
		}

	} else {
		// --- STRATEGY: READ STREAM ---
		// Used for Small files OR Large files with "Unlimited" output
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()

		res, err := ReadStream(f, r.Config.Head, r.Config.Tail)
		if err != nil {
			// If ReadStream fails (e.g. token too long/binary), return error or treat as binary?
			// lx usually returns error here.
			return false, fmt.Errorf("read stream %q: %w", path, err)
		}

		totalRows = res.TotalRows
		isBin = !isExplicitCompact && len(res.HeadBytes) > 0 && IsBinary(res.HeadBytes)
		if res.TotalRows == 0 && len(res.HeadBytes) == 0 {
			isExplicitCompact = true
		}

		if !isBin && !isExplicitCompact {
			view = res.HeadBytes

			// Append Gap if needed
			if r.Config.Head > 0 && r.Config.Tail > 0 {
				skipped := totalRows - r.Config.Head - r.Config.Tail
				if skipped > 0 {
					view = append(view, []byte(fmt.Sprintf("... (%d rows skipped)\n", skipped))...)
				}
			}

			view = append(view, res.TailBytes...)
			language = DetectLanguage(path, res.HeadBytes)

			if r.Config.LineNumbers {
				// Note: Numbering logic in lines.go requires valid slices.
				// Since we constructed view manually, addLineNumbers will re-split it.
				view = addLineNumbers(view, totalRows, r.Config.Head, r.Config.Tail)
			}
		}
	}

	ctx := FileContext{
		Path:          path,
		Size:          fileSize,
		ModTime:       info.ModTime(),
		TotalRows:     totalRows,
		IsEstimate:    isEstimate,
		Language:      language,
		Content:       string(view),
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
