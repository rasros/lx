package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/rasros/lx/pkg/lx/internal/content"
	"github.com/rasros/lx/pkg/lx/internal/detect"
)

type RenderedItem struct {
	Body          string
	IsCompactView bool
}

type Processor struct {
	engine           *TemplateEngine
	global           GlobalContext
	tokenCounter     TokenCounter
	hasRenderedFirst bool
	lastWasCompact   bool
}

func NewProcessor(engine *TemplateEngine, global GlobalContext) *Processor {
	return &Processor{
		engine:       engine,
		global:       global,
		tokenCounter: DefaultTokenCounter,
	}
}

// RenderPrepared processes a preparedItem which contains pre-calculated context
func (p *Processor) RenderPrepared(w io.Writer, item preparedItem, scratchBuf []byte) error {
	var isCompact bool
	var err error
	var ctx interface{}
	var templateToUse *template.Template

	switch v := item.raw.(type) {
	case InputFile:
		var fCtx FileContext
		fCtx, err = p.prepareFileContext(v, item.fileIndexGlobal, scratchBuf)
		if err != nil {
			return err
		}
		fCtx.Section = *item.section
		fCtx.SectionFileIndex = item.fileIndexSection

		isCompact = fCtx.IsCompactView
		ctx = &fCtx
		templateToUse = p.engine.Main

	case SectionContext:
		v = *item.section
		isCompact = false
		ctx = &v
		templateToUse = p.engine.Section

	case PromptContext:
		v.Global = p.global
		v.Section = *item.section
		isCompact = false
		ctx = &v
		templateToUse = p.engine.Prompt

	default:
		return nil
	}

	switch c := ctx.(type) {
	case *FileContext:
		c.Separator = ""
	case *SectionContext:
		c.Separator = ""
	case *PromptContext:
		c.Separator = ""
	}

	if err := templateToUse.Execute(w, ctx); err != nil {
		return err
	}

	p.hasRenderedFirst = true
	p.lastWasCompact = isCompact
	return nil
}

func (p *Processor) prepareFileContext(file InputFile, index int, scratch []byte) (FileContext, error) {
	rc, err := file.Open()
	if err != nil {
		return FileContext{}, err
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

	if scratch == nil {
		scratch = make([]byte, 1024)
	}
	headerLen := 1024
	if len(scratch) < headerLen {
		headerLen = len(scratch)
	}
	n, _ := reader.ReadAt(scratch[:headerLen], 0)
	isBinary := detect.IsBinary(scratch[:n])
	lang := detect.DetectLanguage(file.Path, scratch[:n])
	isImage := detect.IsImage(file.Path)
	cfg := file.Config
	isCompact := (cfg.Head == 0 && cfg.Tail == 0) || isBinary || size == 0
	totalRows, exact, _ := content.EstimateLineCount(reader, size, scratch)

	var contentData interface{}
	if !isBinary && !isCompact && size > 0 && !isImage {
		head, tail, gap, err := p.readSlices(reader, size, totalRows, !exact, cfg)
		if err == nil {
			contentData = p.formatContent(head, tail, gap, totalRows, cfg)
		}
	}

	return FileContext{
		Path:          file.Path,
		AbsPath:       file.AbsPath,
		Size:          size,
		TotalRows:     totalRows,
		IsEstimate:    !exact,
		Language:      lang,
		Content:       contentData,
		TokenEstimate: p.tokenCounter(size, contentData),
		IsBinary:      isBinary,
		IsImage:       isImage,
		IsCompactView: isCompact,
		FileIndex:     index,
		Global:        p.global,
	}, nil
}

func (p *Processor) readSlices(reader io.ReaderAt, size int64, totalRows int, isEstimate bool, cfg RunnerConfig) (head, tail, gap []byte, err error) {
	if cfg.Head < 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, -1)
		return head, nil, nil, err
	}
	if cfg.Head > 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = content.ReadHead(sr, cfg.Head)
	}
	if cfg.Tail > 0 {
		skipped := totalRows - cfg.Head - cfg.Tail
		if skipped > 0 && cfg.Head > 0 {
			tilde := ""
			if isEstimate {
				tilde = "~"
			}
			gap = []byte(fmt.Sprintf("... (%s%d rows skipped)\n", tilde, skipped))
		}
		if f, ok := reader.(*os.File); ok {
			tail, _ = content.ReadTailSeek(f, cfg.Tail)
		} else if br, ok := reader.(*bytes.Reader); ok {
			tail, _ = tailFromBuffer(br, cfg.Tail)
		}
	}
	return
}

func (p *Processor) formatContent(head, tail, gap []byte, totalRows int, cfg RunnerConfig) interface{} {
	if cfg.LineNumbers {
		return content.LineNumberFormatter{Head: head, Gap: gap, Tail: tail, TotalRows: totalRows}
	}
	var res bytes.Buffer
	res.Write(head)
	res.Write(gap)
	res.Write(tail)
	return res.String()
}

func tailFromBuffer(r *bytes.Reader, lines int) ([]byte, error) {
	data := make([]byte, r.Size())
	_, _ = r.ReadAt(data, 0)
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			count++
			if count > lines {
				return data[i+1:], nil
			}
		}
	}
	return data, nil
}
