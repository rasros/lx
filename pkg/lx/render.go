package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func Humanize(s int64) string {
	return TemplateFuncs()["humanize"].(func(int64) string)(s)
}

// EstimateTokens returns a rough estimate of the number of tokens (approx 4 chars/token).
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

func (r *Runner) RunFile(file InputFile, index int, prevCompact bool, currentSection int, out io.Writer) (bool, error) {
	log := r.Global.Config.Logger
	if log != nil {
		log.Debugf("[%d] processing file: %s", index, file.Path)
	}

	reader, fileSize, displayPath, closeFunc, err := r.openInput(file)
	if err != nil {
		return false, err
	}
	defer closeFunc()

	isBinary, isImage, language := r.detectAttributes(displayPath, reader, fileSize)

	isExplicitCompact := r.Config.Head == 0 && r.Config.Tail == 0
	isCompactView := isExplicitCompact || fileSize == 0

	var (
		headBytes, tailBytes, gapBytes []byte
		totalRows                      int
		isEstimate                     bool
		content                        interface{}
	)

	// Process text content if not binary/image
	if !isBinary && fileSize > 0 {
		var exact bool
		totalRows, exact, err = EstimateLineCount(reader, fileSize)
		if err != nil && log != nil {
			log.Warnf("failed to count lines for %s: %v", displayPath, err)
		}
		isEstimate = !exact

		if !isExplicitCompact {
			headBytes, tailBytes, gapBytes, err = r.readContentSlice(reader, fileSize, totalRows, isEstimate)
			if err != nil {
				return false, err
			}

			if len(headBytes) > 0 {
				language = DetectLanguage(displayPath, headBytes)
			}
		}
	}

	if !isBinary && !isExplicitCompact && fileSize > 0 {
		content = r.formatContent(headBytes, tailBytes, gapBytes, totalRows)
	}

	if prevCompact && !isCompactView {
		out.Write([]byte("\n"))
	}

	ctx := FileContext{
		Path:           displayPath,
		AbsPath:        file.AbsPath,
		Size:           fileSize,
		ModTime:        file.ModTime,
		TotalRows:      totalRows,
		TokenEstimate:  EstimateTokens(fileSize),
		IsEstimate:     isEstimate,
		Language:       language,
		Content:        content,
		IsBinary:       isBinary,
		IsImage:        isImage,
		IsCompactView:  isCompactView,
		FileIndex:      index,
		CurrentSection: currentSection,
		Global:         r.Global,
	}

	if err := r.Engine.Main.Execute(out, ctx); err != nil {
		return false, fmt.Errorf("template exec: %w", err)
	}

	return ctx.IsCompactView, nil
}

func (r *Runner) openInput(file InputFile) (reader io.ReaderAt, size int64, path string, cleanup func(), err error) {
	path = file.Path
	size = file.Size

	rc, err := file.Open()
	if err != nil {
		return nil, 0, "", nil, err
	}

	cleanup = func() { rc.Close() }

	if path == "-" || path == "stdin" {
		path = "stdin"
		data, err := io.ReadAll(rc)
		if err != nil {
			cleanup()
			return nil, 0, "", nil, err
		}
		reader = bytes.NewReader(data)
		size = int64(len(data))
		cleanup = func() {}
	} else if f, ok := rc.(*os.File); ok {
		reader = f
	} else {
		// Buffer non-seekable streams to allow random access
		if r.Global.Config.Logger != nil {
			r.Global.Config.Logger.Debugf("buffering stream for random access: %s", path)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			cleanup()
			return nil, 0, "", nil, err
		}
		reader = bytes.NewReader(data)
		size = int64(len(data))
		cleanup = func() {}
	}

	return reader, size, path, cleanup, nil
}

func (r *Runner) detectAttributes(path string, reader io.ReaderAt, size int64) (isBinary, isImage bool, language string) {
	isImage = IsImage(path)

	if size == 0 {
		return false, isImage, ""
	}

	header := make([]byte, 1024)
	n, _ := reader.ReadAt(header, 0)
	header = header[:n]

	isBinary = IsBinary(header)

	if !isBinary {
		language = DetectLanguage(path, header)
	}

	log := r.Global.Config.Logger
	if log != nil {
		if isBinary {
			log.Debugf("binary file detected: %s", path)
		} else if language != "" {
			log.Debugf("language detected: %s (%s)", language, path)
		}
	}

	return isBinary, isImage, language
}

func (r *Runner) readContentSlice(reader io.ReaderAt, size int64, totalRows int, isEstimate bool) (head, tail, gap []byte, err error) {
	// Read everything
	if r.Config.Head < 0 && r.Config.Tail < 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = ReadHead(sr, -1)
		return head, nil, nil, err
	}

	if r.Config.Head > 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = ReadHead(sr, r.Config.Head)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if r.Config.Tail > 0 {
		skipped := totalRows - r.Config.Head - r.Config.Tail
		if skipped > 0 && r.Config.Head > 0 {
			tilde := ""
			if isEstimate {
				tilde = "~"
			}
			gap = []byte(fmt.Sprintf("... (%s%d rows skipped)\n", tilde, skipped))
		}

		if f, ok := reader.(*os.File); ok {
			tail, err = ReadTailSeek(f, r.Config.Tail)
		} else if br, ok := reader.(*bytes.Reader); ok {
			allData := make([]byte, size)
			br.ReadAt(allData, 0)
			tail = tailFromBuffer(allData, r.Config.Tail)
		}

		if err != nil {
			return nil, nil, nil, err
		}
	}

	return head, tail, gap, nil
}

func (r *Runner) formatContent(head, tail, gap []byte, totalRows int) interface{} {
	if r.Config.LineNumbers {
		return LineNumberFormatter{
			Head:      head,
			Gap:       gap,
			Tail:      tail,
			TotalRows: totalRows,
		}
	}

	if len(tail) == 0 && len(gap) == 0 {
		return string(head)
	}

	totalLen := len(head) + len(gap) + len(tail)
	buf := make([]byte, 0, totalLen)
	buf = append(buf, head...)
	buf = append(buf, gap...)
	buf = append(buf, tail...)
	return string(buf)
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
