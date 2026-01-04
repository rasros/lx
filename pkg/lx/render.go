package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// EstimateTokens returns a rough estimate of the number of tokens in the given size.
// Currently uses the heuristic: ~4 characters (bytes) per token.
func EstimateTokens(size int64) int64 {
	if size <= 0 {
		return 0
	}
	return size / 4
}

type Runner struct {
	Config RunnerConfig
	Engine *TemplateEngine
	Global GlobalContext
}

func NewRunner(cfg RunnerConfig, engine *TemplateEngine, global GlobalContext) *Runner {
	return &Runner{
		Config: cfg,
		Engine: engine,
		Global: global,
	}
}

func (r *Runner) RunSection(body string, out io.Writer) error {
	ctx := SectionContext{
		Body:   body,
		Global: r.Global,
	}
	return r.Engine.Section.Execute(out, ctx)
}

func (r *Runner) RunPrompt(body string, out io.Writer) error {
	ctx := PromptContext{
		Body:   body,
		Global: r.Global,
	}
	return r.Engine.Prompt.Execute(out, ctx)
}

func (r *Runner) RunFile(path string, index int, prevCompact bool, out io.Writer) (bool, error) {
	var (
		contentReader io.ReaderAt
		fileSize      int64
		modTime       time.Time
		absPath       string
		isStdin       bool
	)

	if path == "-" {
		isStdin = true
		absPath = "stdin"
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return false, fmt.Errorf("read stdin: %w", err)
		}
		contentReader = bytes.NewReader(data)
		fileSize = int64(len(data))
		modTime = time.Now()
	} else {
		info, err := os.Stat(path)
		if err != nil {
			return false, fmt.Errorf("stat %q: %w", path, err)
		}
		fileSize = info.Size()
		modTime = info.ModTime()

		abs, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		} else {
			absPath = abs
		}

		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		contentReader = f
	}

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

	if isExplicitCompact {
		var exact bool
		totalRows, exact, _ = EstimateLineCount(contentReader, fileSize)
		isEstimate = !exact
	} else {
		header := make([]byte, 1024)
		n, _ := contentReader.ReadAt(header, 0)
		isBin = IsBinary(header[:n])

		if !isBin {
			var reader io.ReadSeeker
			if isStdin {
				reader = contentReader.(*bytes.Reader)
			} else {
				reader = contentReader.(*os.File)
			}

			if isUnlimited {
				reader.Seek(0, 0)
				headBytes, totalRows, _ = ReadHead(reader, -1)
			} else {
				var exact bool
				totalRows, exact, _ = EstimateLineCount(contentReader, fileSize)
				isEstimate = !exact

				if r.Config.Head > 0 {
					reader.Seek(0, 0)
					headBytes, _, _ = ReadHead(reader, r.Config.Head)
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
					if isStdin {
						rdr := contentReader.(*bytes.Reader)
						allData := make([]byte, fileSize)
						rdr.ReadAt(allData, 0)
						tailBytes = tailFromBuffer(allData, r.Config.Tail)
					} else {
						tailBytes, _ = ReadTailSeek(contentReader.(*os.File), r.Config.Tail)
					}
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
		AbsPath:       absPath,
		Size:          fileSize,
		ModTime:       modTime,
		TotalRows:     totalRows,
		TokenEstimate: EstimateTokens(fileSize),
		IsEstimate:    isEstimate,
		Language:      language,
		Content:       content,
		IsBinary:      isBin,
		IsCompactView: isExplicitCompact || isBin || fileSize == 0,
		FileIndex:     index,
		Global:        r.Global,
	}

	if err := r.Engine.Main.Execute(out, ctx); err != nil {
		return false, fmt.Errorf("template exec: %w", err)
	}

	return ctx.IsCompactView, nil
}

func tailFromBuffer(data []byte, lines int) []byte {
	if lines <= 0 || len(data) == 0 {
		return nil
	}
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			count++
			if i < len(data)-1 && count >= lines {
			}
		}
	}
	newlinesFound := 0
	start := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			newlinesFound++
			if newlinesFound > lines {
				start = i + 1
				break
			}
		}
	}
	return data[start:]
}
