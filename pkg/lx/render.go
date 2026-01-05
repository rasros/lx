package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
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

// RunFile now accepts the abstract InputFile
func (r *Runner) RunFile(file InputFile, index int, prevCompact bool, out io.Writer) (bool, error) {
	var (
		contentReader io.ReaderAt
		fileSize      int64     = file.Size
		modTime       time.Time = file.ModTime
		path          string    = file.Path
		absPath       string    = file.AbsPath
	)

	if path == "-" || path == "stdin" {
		// Stdin content is buffered in memory to support random access (ReaderAt),
		// which is required for line counting, slicing, and token estimation.
		rc, err := file.Open()
		if err != nil {
			return false, err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return false, err
		}
		contentReader = bytes.NewReader(data)
		fileSize = int64(len(data))

		// Ensure it is formatted as "stdin" in output
		path = "stdin"
	} else {
		// For standard files (and Archives later), we need ReadAt for head/tail.
		// The generic Open() returns ReadCloser.
		// If it's an OS file, we can cast.
		// If it's a Zip entry, we might need to read all if it doesn't support Seek.

		rc, err := file.Open()
		if err != nil {
			return false, err
		}

		// Attempt to upgrade to ReaderAt/Seeker
		if f, ok := rc.(*os.File); ok {
			contentReader = f
			defer f.Close()
		} else {
			// Fallback for non-OS files (e.g. Zip streams): Read All into buffer
			// because our Slicing logic depends on ReadAt.
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return false, err
			}
			contentReader = bytes.NewReader(data)
		}
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
			if rdr, ok := contentReader.(io.ReadSeeker); ok {
				reader = rdr
			} else {
				// Should not happen with bytes.Reader or os.File
				return false, fmt.Errorf("reader does not support seeking")
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

					// Tail logic
					if f, ok := contentReader.(*os.File); ok {
						tailBytes, _ = ReadTailSeek(f, r.Config.Tail)
					} else {
						// For bytes.Reader (Stdin or Zip buffer)
						rdr := contentReader.(*bytes.Reader)
						allData := make([]byte, fileSize)
						rdr.ReadAt(allData, 0)
						tailBytes = tailFromBuffer(allData, r.Config.Tail)
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
	// (Existing tail logic is fine)
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
