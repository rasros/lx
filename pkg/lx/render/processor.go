package render

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/skeleton"
	"github.com/rasros/lx/pkg/lx/sources"
)

// FileErrorHandler handles per-file read errors.
type FileErrorHandler func(f sources.InputFile, err error)

// PreparedItem carries precomputed context for deterministic render order.
type PreparedItem struct {
	Raw              interface{}
	Section          *core.SectionContext
	FileIndexGlobal  int
	FileIndexSection int
	StreamIndex      int
}

// Processor renders prepared items into an output writer.
type Processor struct {
	engine           *core.TemplateEngine
	global           core.GlobalContext
	tokenCounter     core.TokenCounter
	onFileError      FileErrorHandler
	lastWasCompact   bool
	format           string
	extractDocuments bool
}

func NewProcessor(engine *core.TemplateEngine, global core.GlobalContext, onError FileErrorHandler, format string, extractDocuments bool) *Processor {
	return &Processor{
		engine:           engine,
		global:           global,
		onFileError:      onError,
		tokenCounter:     core.DefaultTokenCounter,
		format:           format,
		extractDocuments: extractDocuments,
	}
}

func (p *Processor) SetTokenCounter(tc core.TokenCounter) {
	if tc != nil {
		p.tokenCounter = tc
	}
}

func (p *Processor) LastWasCompact() bool { return p.lastWasCompact }

// RenderPrepared processes one item containing pre-calculated section and indices.
func (p *Processor) RenderPrepared(w io.Writer, item PreparedItem, scratchBuf []byte) error {
	var isCompact bool
	var err error
	var ctx interface{}
	var templateToUse *template.Template

	switch v := item.Raw.(type) {
	case sources.InputFile:
		var fCtx core.FileContext
		fCtx, err = p.prepareFileContext(v, item.FileIndexGlobal, scratchBuf)
		if err != nil {
			return err
		}
		fCtx.Section = *item.Section
		fCtx.SectionFileIndex = item.FileIndexSection

		isCompact = fCtx.IsCompactView
		ctx = &fCtx

		if fCtx.IsError {
			templateToUse = p.engine.FileError
		} else if fCtx.IsImage {
			if p.format == "html" {
				templateToUse = p.engine.FileContent
			} else {
				templateToUse = p.engine.FileBinary
			}
		} else if fCtx.IsBinary {
			templateToUse = p.engine.FileBinary
		} else if fCtx.IsCompactView {
			templateToUse = p.engine.FileCompact
		} else {
			templateToUse = p.engine.FileContent
		}

	case core.SectionContext:
		v = *item.Section
		ctx = &v
		templateToUse = p.engine.Section

	case core.PromptContext:
		v.Global = p.global
		v.Section = *item.Section
		ctx = &v
		templateToUse = p.engine.Prompt

	case core.TreeContext:
		v.Global = p.global
		v.Section = *item.Section
		ctx = &v
		templateToUse = p.engine.Tree

	default:
		return nil
	}

	if err := templateToUse.Execute(w, ctx); err != nil {
		return err
	}

	p.lastWasCompact = isCompact
	return nil
}

func (p *Processor) prepareFileContext(file sources.InputFile, index int, scratch []byte) (core.FileContext, error) {
	rc, err := file.Open()
	if err != nil {
		if p.onFileError != nil {
			p.onFileError(file, err)
		}
		return core.FileContext{
			Path:      file.Path,
			AbsPath:   file.AbsPath,
			Size:      file.Size,
			ModTime:   file.ModTime,
			FileIndex: index,
			Global:    p.global,
			ReadError: err.Error(),
			IsError:   true,
		}, nil
	}
	defer rc.Close()

	if f, ok := rc.(*os.File); ok {
		if stat, err := f.Stat(); err == nil && stat.IsDir() {
			return core.FileContext{
				Path:      file.Path,
				AbsPath:   file.AbsPath,
				Size:      file.Size,
				ModTime:   file.ModTime,
				FileIndex: index,
				Global:    p.global,
				ReadError: "is a directory",
				IsError:   true,
			}, nil
		}
	}

	var reader io.ReaderAt
	var size int64
	if f, ok := rc.(*os.File); ok {
		reader, size = f, file.Size
	} else {
		data, _ := io.ReadAll(rc)
		reader, size = bytes.NewReader(data), int64(len(data))
	}

	if p.extractDocuments && sources.IsDocumentPath(file.Path) {
		if text, err := sources.ExtractDocumentText(file.Path, reader, size); err == nil {
			reader = bytes.NewReader(text)
			size = int64(len(text))
		}
	}

	if scratch == nil {
		scratch = make([]byte, 1024)
	}
	headerLen := 1024
	if len(scratch) < headerLen {
		headerLen = len(scratch)
	}

	n, _ := reader.ReadAt(scratch[:headerLen], 0)
	isBinary := internal.IsBinary(scratch[:n])
	lang := internal.DetectLanguage(file.Path, scratch[:n])
	isImage := internal.IsImage(file.Path)
	cfg := file.Config

	var skeletonMode string
	if !isBinary && !isImage && (cfg.SkeletonFunctions || cfg.SkeletonTypes) && skeleton.Supported(lang) {
		allData := make([]byte, size)
		if _, err := reader.ReadAt(allData, 0); err == nil {
			filtered := skeleton.Extract(lang, allData, cfg.SkeletonFunctions, cfg.SkeletonTypes)
			reader = bytes.NewReader(filtered)
			size = int64(len(filtered))
			if int64(headerLen) > size {
				headerLen = int(size)
			}
			n, _ = reader.ReadAt(scratch[:headerLen], 0)
			switch {
			case cfg.SkeletonFunctions && cfg.SkeletonTypes:
				skeletonMode = "definitions"
			case cfg.SkeletonFunctions:
				skeletonMode = "function signatures"
			default:
				skeletonMode = "type definitions"
			}
		}
	}

	isCompact := (cfg.Head == 0 && cfg.Tail == 0) || isBinary || size == 0
	totalRows, exact, _ := internal.EstimateLineCount(reader, size, scratch)

	var contentData interface{}
	if !isBinary && !isCompact && size > 0 && !isImage {
		effectiveCfg := cfg
		if exact && cfg.Head > 0 && cfg.Tail > 0 && totalRows <= (cfg.Head+cfg.Tail) {
			effectiveCfg.Head = -1
			effectiveCfg.Tail = 0
		}

		head, tail, gap, err := p.readSlices(reader, size, totalRows, !exact, effectiveCfg)
		if err != nil {
			return core.FileContext{
				Path:      file.Path,
				AbsPath:   file.AbsPath,
				Size:      file.Size,
				ModTime:   file.ModTime,
				FileIndex: index,
				Global:    p.global,
				ReadError: err.Error(),
				IsError:   true,
			}, nil
		}
		contentData = p.formatContent(head, tail, gap, totalRows, effectiveCfg)
	}

	return core.FileContext{
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
		SkeletonMode:  skeletonMode,
	}, nil
}

func (p *Processor) readSlices(reader io.ReaderAt, size int64, totalRows int, isEstimate bool, cfg core.RunnerConfig) (head, tail, gap []byte, err error) {
	if cfg.Head < 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = internal.ReadHead(sr, -1)
		return head, nil, nil, err
	}
	if cfg.Head > 0 {
		sr := io.NewSectionReader(reader, 0, size)
		head, _, err = internal.ReadHead(sr, cfg.Head)
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
			tail, _ = internal.ReadTailSeek(f, cfg.Tail)
		} else if br, ok := reader.(*bytes.Reader); ok {
			tail, _ = tailFromBuffer(br, cfg.Tail)
		}
	}
	return
}

func (p *Processor) formatContent(head, tail, gap []byte, totalRows int, cfg core.RunnerConfig) interface{} {
	if cfg.LineNumbers {
		return internal.LineNumberFormatter{Head: head, Gap: gap, Tail: tail, TotalRows: totalRows}
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
