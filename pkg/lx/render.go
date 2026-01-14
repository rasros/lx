package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/rasros/lx/pkg/lx/internal/content"
	"github.com/rasros/lx/pkg/lx/internal/detect"
)

type RenderedItem struct {
	Body          string
	IsCompactView bool
}

type Processor struct {
	engine       *TemplateEngine
	cfg          RunnerConfig
	global       GlobalContext
	tokenCounter TokenCounter
}

func NewProcessor(engine *TemplateEngine, cfg RunnerConfig, global GlobalContext) *Processor {
	return &Processor{
		engine:       engine,
		cfg:          cfg,
		global:       global,
		tokenCounter: DefaultTokenCounter,
	}
}

func (p *Processor) Render(w io.Writer, item StreamItem, index int) error {
	switch v := item.(type) {
	case InputFile:
		rendered, err := p.renderFile(v, index)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(w, rendered.Body)
		return err
	case SectionContext:
		v.Global = p.global // Sync global state
		return p.engine.Section.Execute(w, v)
	case PromptContext:
		v.Global = p.global // Sync global state
		return p.engine.Prompt.Execute(w, v)
	default:
		return nil
	}
}

func (p *Processor) renderFile(file InputFile, index int) (RenderedItem, error) {
	rc, err := file.Open()
	if err != nil {
		return RenderedItem{}, err
	}
	defer rc.Close()

	var reader io.ReaderAt
	var size int64
	if f, ok := rc.(*os.File); ok {
		reader, size = f, file.Size
	} else {
		data, _ := io.ReadAll(rc)
		reader, size = bytes.NewReader(data), int64(len(data))
	}

	header := make([]byte, 1024)
	n, _ := reader.ReadAt(header, 0)
	isBinary := detect.IsBinary(header[:n])
	lang := detect.DetectLanguage(file.Path, header[:n])

	isCompact := p.cfg.Head == 0 && p.cfg.Tail == 0
	totalRows, exact, _ := content.EstimateLineCount(reader, size)

	var contentData interface{}
	if !isBinary && !isCompact && size > 0 {
		head, tail, gap, err := p.readSlices(reader, size, totalRows, !exact)
		if err == nil {
			contentData = p.formatContent(head, tail, gap, totalRows)
		}
	}

	ctx := FileContext{
		Path:          file.Path,
		AbsPath:       file.AbsPath,
		Size:          size,
		TotalRows:     totalRows,
		IsEstimate:    !exact,
		Language:      lang,
		Content:       contentData,
		TokenEstimate: p.tokenCounter(size, contentData),
		IsBinary:      isBinary,
		IsCompactView: isCompact,
		FileIndex:     index,
		Global:        p.global,
	}

	var buf bytes.Buffer
	err = p.engine.Main.Execute(&buf, ctx)
	return RenderedItem{Body: buf.String(), IsCompactView: isCompact}, err
}

func (p *Processor) readSlices(reader io.ReaderAt, size int64, totalRows int, isEstimate bool) (head, tail, gap []byte, err error) {
	if p.cfg.Head < 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, -1)
		return head, nil, nil, err
	}

	if p.cfg.Head > 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, p.cfg.Head)
	}

	if p.cfg.Tail > 0 {
		skipped := totalRows - p.cfg.Head - p.cfg.Tail
		if skipped > 0 && p.cfg.Head > 0 {
			tilde := ""
			if isEstimate {
				tilde = "~"
			}
			gap = []byte(fmt.Sprintf("... (%s%d rows skipped)\n", tilde, skipped))
		}

		if f, ok := reader.(*os.File); ok {
			tail, _ = content.ReadTailSeek(f, p.cfg.Tail)
		} else if br, ok := reader.(*bytes.Reader); ok {
			tail = tailFromBuffer(br, p.cfg.Tail)
		}
	}
	return
}

func (p *Processor) formatContent(head, tail, gap []byte, totalRows int) interface{} {
	if p.cfg.LineNumbers {
		return content.LineNumberFormatter{
			Head: head, Gap: gap, Tail: tail, TotalRows: totalRows,
		}
	}
	var res bytes.Buffer
	res.Write(head)
	res.Write(gap)
	res.Write(tail)
	return res.String()
}

func tailFromBuffer(r *bytes.Reader, lines int) []byte {
	data := make([]byte, r.Size())
	r.ReadAt(data, 0)
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			count++
			if count > lines {
				return data[i+1:]
			}
		}
	}
	return data
}
