package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

func Humanize(s int64) string {
	return TemplateFuncs()["humanize"].(func(int64) string)(s)
}

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

func (r *Runner) RunSection(body string, section int, out io.Writer) error {
	ctx := SectionContext{
		Body:    body,
		Section: section,
		Global:  r.Global,
	}
	return r.Engine.Section.Execute(out, ctx)
}

func (r *Runner) RunPrompt(body string, section int, out io.Writer) error {
	ctx := PromptContext{
		Body:    body,
		Section: section,
		Global:  r.Global,
	}
	return r.Engine.Prompt.Execute(out, ctx)
}

// RunFile now accepts the abstract InputFile
func (r *Runner) RunFile(file InputFile, index int, prevCompact bool, currentSection int, out io.Writer) (bool, error) {
	log := r.Global.Config.Logger
	if log != nil {
		log.Debugf("[%d] processing file: %s", index, file.Path)
	}

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
		path = "stdin"
	} else {
		rc, err := file.Open()
		if err != nil {
			return false, err
		}

		if f, ok := rc.(*os.File); ok {
			contentReader = f
			defer f.Close()
		} else {
			// Fallback for non-seekable streams: buffer all
			if log != nil {
				log.Debugf("buffering stream for random access: %s", path)
			}
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

	if fileSize > 0 {
		header := make([]byte, 1024)
		n, _ := contentReader.ReadAt(header, 0)
		isBin = IsBinary(header[:n])

		if isBin {
			if log != nil {
				log.Infof("binary file detected: %s", path)
			}
		} else if n > 0 {
			language = DetectLanguage(path, header[:n])
			if log != nil && language != "" {
				log.Debugf("language detected: %s (%s)", language, path)
			}
		}
	}

	if isExplicitCompact {
		if !isBin {
			var exact bool
			totalRows, exact, _ = EstimateLineCount(contentReader, fileSize)
			isEstimate = !exact
			if log != nil {
				log.Debugf("compact view line count (estimate=%v): %d", isEstimate, totalRows)
			}
		}
	} else if !isBin {
		var reader io.ReadSeeker
		if rdr, ok := contentReader.(io.ReadSeeker); ok {
			reader = rdr
		} else {
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
				if log != nil {
					log.Debugf("reading head: %d lines", r.Config.Head)
				}
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

				if log != nil {
					log.Debugf("reading tail: %d lines", r.Config.Tail)
				}
				if f, ok := contentReader.(*os.File); ok {
					tailBytes, _ = ReadTailSeek(f, r.Config.Tail)
				} else {
					rdr := contentReader.(*bytes.Reader)
					allData := make([]byte, fileSize)
					rdr.ReadAt(allData, 0)
					tailBytes = tailFromBuffer(allData, r.Config.Tail)
				}
			}
		}

		if len(headBytes) > 0 {
			language = DetectLanguage(path, headBytes)
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
		Path:           path,
		AbsPath:        absPath,
		Size:           fileSize,
		ModTime:        modTime,
		TotalRows:      totalRows,
		TokenEstimate:  EstimateTokens(fileSize),
		IsEstimate:     isEstimate,
		Language:       language,
		Content:        content,
		IsBinary:       isBin,
		IsCompactView:  isExplicitCompact || isBin || fileSize == 0,
		FileIndex:      index,
		CurrentSection: currentSection,
		Global:         r.Global,
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
