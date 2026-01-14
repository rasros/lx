package lx

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/rasros/lx/pkg/lx/internal/content"
	"github.com/rasros/lx/pkg/lx/internal/detect"
)

type RenderedItem struct {
	Body          string
	IsCompactView bool
}

type Runner struct {
	Config RunnerConfig
	Engine *TemplateEngine
	Global GlobalContext
	Logger *slog.Logger
}

func NewRunner(cfg RunnerConfig, engine *TemplateEngine, global GlobalContext, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Runner{
		Config: cfg,
		Engine: engine,
		Global: global,
		Logger: logger,
	}
}

func (r *Runner) RunSection(body string, section int) (RenderedItem, error) {
	ctx := SectionContext{
		Body:    body,
		Section: section,
		Global:  r.Global,
	}
	var buf bytes.Buffer
	err := r.Engine.Section.Execute(&buf, ctx)
	return RenderedItem{Body: buf.String(), IsCompactView: false}, err
}

func (r *Runner) RunPrompt(body string, section int) (RenderedItem, error) {
	ctx := PromptContext{
		Body:    body,
		Section: section,
		Global:  r.Global,
	}
	var buf bytes.Buffer
	err := r.Engine.Prompt.Execute(&buf, ctx)
	return RenderedItem{Body: buf.String(), IsCompactView: false}, err
}

func (r *Runner) RunFile(file InputFile, index int, currentSection int) (RenderedItem, error) {
	r.Logger.Debug("processing file", "index", index, "path", file.Path)

	reader, fileSize, displayPath, closeFunc, err := r.openInput(file)
	if err != nil {
		return RenderedItem{}, err
	}
	defer closeFunc()

	isBinary, isImage, language := r.detectAttributes(displayPath, reader, fileSize)

	isExplicitCompact := r.Config.Head == 0 && r.Config.Tail == 0
	isCompactView := isExplicitCompact || fileSize == 0

	var (
		headBytes, tailBytes, gapBytes []byte
		totalRows                      int
		isEstimate                     bool
		contentData                    interface{}
	)

	if !isBinary && fileSize > 0 {
		var exact bool
		totalRows, exact, err = content.EstimateLineCount(reader, fileSize)
		if err != nil {
			r.Logger.Warn("failed to count lines", "path", displayPath, "error", err)
		}
		isEstimate = !exact

		if !isExplicitCompact {
			headBytes, tailBytes, gapBytes, err = r.readContentSlice(reader, fileSize, totalRows, isEstimate)
			if err != nil {
				return RenderedItem{}, err
			}

			if len(headBytes) > 0 {
				language = detect.DetectLanguage(displayPath, headBytes)
			}
		}
	}

	if !isBinary && !isExplicitCompact && fileSize > 0 {
		contentData = r.formatContent(headBytes, tailBytes, gapBytes, totalRows)
	}

	ctx := FileContext{
		Path:           displayPath,
		AbsPath:        file.AbsPath,
		Size:           fileSize,
		ModTime:        file.ModTime,
		TotalRows:      totalRows,
		TokenEstimate:  fileSize / 4,
		IsEstimate:     isEstimate,
		Language:       language,
		Content:        contentData,
		IsBinary:       isBinary,
		IsImage:        isImage,
		IsCompactView:  isCompactView,
		FileIndex:      index,
		CurrentSection: currentSection,
		Global:         r.Global,
	}

	var buf bytes.Buffer
	if err := r.Engine.Main.Execute(&buf, ctx); err != nil {
		return RenderedItem{}, fmt.Errorf("template exec: %w", err)
	}

	return RenderedItem{Body: buf.String(), IsCompactView: isCompactView}, nil
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
	isImage = detect.IsImage(path)
	if size == 0 {
		return false, isImage, ""
	}
	header := make([]byte, 1024)
	n, _ := reader.ReadAt(header, 0)
	header = header[:n]
	isBinary = detect.IsBinary(header)
	if !isBinary {
		language = detect.DetectLanguage(path, header)
	}
	return isBinary, isImage, language
}

func (r *Runner) readContentSlice(reader io.ReaderAt, size int64, totalRows int, isEstimate bool) (head, tail, gap []byte, err error) {
	if r.Config.Head < 0 && r.Config.Tail < 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, -1)
		return head, nil, nil, err
	}
	if r.Config.Head > 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, r.Config.Head)
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
			tail, err = content.ReadTailSeek(f, r.Config.Tail)
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
		return content.LineNumberFormatter{
			Head:      head,
			Gap:       gap,
			Tail:      tail,
			TotalRows: totalRows,
		}
	}
	if len(tail) == 0 && len(gap) == 0 {
		return string(head)
	}
	return string(head) + string(gap) + string(tail)
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
