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

func (p *Processor) Render(w io.Writer, item StreamItem, index int) error {
	var isCompact bool
	var err error

	// 1. Prepare the context and determine layout properties (IsCompact)
	var ctx interface{}
	var templateToUse *template.Template

	switch v := item.(type) {
	case InputFile:
		var fCtx FileContext
		fCtx, err = p.prepareFileContext(v, index)
		if err != nil {
			return err
		}
		isCompact = fCtx.IsCompactView
		ctx = &fCtx
		templateToUse = p.engine.Main

	case SectionContext:
		v.Global = p.global
		isCompact = false
		ctx = &v
		templateToUse = p.engine.Section

	case PromptContext:
		v.Global = p.global
		isCompact = false
		ctx = &v
		templateToUse = p.engine.Prompt

	default:
		return nil
	}

	// 2. Calculate Separator based on state
	separator := ""
	if p.hasRenderedFirst {
		if p.lastWasCompact && isCompact {
			separator = "\n"
		} else {
			separator = "\n\n"
		}
	}

	// 3. Inject Separator into the context
	switch c := ctx.(type) {
	case *FileContext:
		c.Separator = separator
	case *SectionContext:
		c.Separator = separator
	case *PromptContext:
		c.Separator = separator
	}

	// 4. Execute Template
	if err := templateToUse.Execute(w, ctx); err != nil {
		return err
	}

	// 5. Update State
	p.hasRenderedFirst = true
	p.lastWasCompact = isCompact

	return nil
}

// prepareFileContext reads the file and builds the context without rendering it.
func (p *Processor) prepareFileContext(file InputFile, index int) (FileContext, error) {
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

	header := make([]byte, 1024)
	n, _ := reader.ReadAt(header, 0)
	isBinary := detect.IsBinary(header[:n])
	lang := detect.DetectLanguage(file.Path, header[:n])
	isImage := detect.IsImage(file.Path)

	cfg := file.Config

	// Determine "Compact" status for spacing logic
	// A file is compact if user requested -n0, OR it's binary, OR it's empty.
	isCompact := (cfg.Head == 0 && cfg.Tail == 0) || isBinary || size == 0

	totalRows, exact, _ := content.EstimateLineCount(reader, size)

	var contentData interface{}
	// Only read content slices if we actually plan to show them
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
			tail = tailFromBuffer(br, cfg.Tail)
		}
	}
	return
}

func (p *Processor) formatContent(head, tail, gap []byte, totalRows int, cfg RunnerConfig) interface{} {
	if cfg.LineNumbers {
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
