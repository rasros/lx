package render

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	engine       *core.TemplateEngine
	global       core.GlobalContext
	tokenCounter core.TokenCounter
	onFileError  FileErrorHandler
}

// RenderStats reports what one rendered item turned out to be. A non-file item
// yields the zero value.
type RenderStats struct {
	Rows      int
	IsCompact bool
	IsBinary  bool
	IsError   bool
}

type readerAtWithSize interface {
	io.ReaderAt
	Size() int64
}

type byteReaderProvider interface {
	ByteReader() *bytes.Reader
}

func NewProcessor(engine *core.TemplateEngine, global core.GlobalContext, onError FileErrorHandler) *Processor {
	return &Processor{
		engine:       engine,
		global:       global,
		onFileError:  onError,
		tokenCounter: core.DefaultTokenCounter,
	}
}

func (p *Processor) SetTokenCounter(tc core.TokenCounter) {
	if tc != nil {
		p.tokenCounter = tc
	}
}

// RenderPrepared processes one item containing pre-calculated section and indices.
func (p *Processor) RenderPrepared(w io.Writer, item PreparedItem, scratchBuf []byte) (RenderStats, error) {
	var stats RenderStats
	var ctx interface{}
	var templateToUse *template.Template

	switch v := item.Raw.(type) {
	case sources.InputFile:
		fCtx, err := p.prepareFileContext(v, item.FileIndexGlobal, scratchBuf)
		if err != nil {
			return stats, err
		}
		fCtx.Section = *item.Section
		fCtx.SectionFileIndex = item.FileIndexSection

		stats = RenderStats{
			Rows:      fCtx.TotalRows,
			IsCompact: fCtx.IsCompactView,
			IsBinary:  fCtx.IsBinary,
			IsError:   fCtx.IsError,
		}
		ctx = &fCtx

		if fCtx.IsError {
			templateToUse = p.engine.FileError
		} else if fCtx.IsImage {
			templateToUse = p.engine.FileImage
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

	case core.MetaContext:
		v.Global = p.global
		v.Section = *item.Section
		ctx = &v
		templateToUse = p.engine.Meta

	default:
		return stats, nil
	}

	if err := templateToUse.Execute(w, ctx); err != nil {
		return stats, err
	}
	return stats, nil
}

// errorContext describes a file that could not be read.
func (p *Processor) errorContext(file sources.InputFile, index int, msg string) core.FileContext {
	return core.FileContext{
		Path:         file.Path,
		AbsPath:      file.AbsPath,
		Size:         file.Size,
		OriginalSize: file.Size,
		ModTime:      file.ModTime,
		FileIndex:    index,
		Global:       p.global,
		ReadError:    msg,
		IsError:      true,
	}
}

// readerFor adapts an opened file to random access, buffering the content only
// when the reader cannot provide it directly. Cases are ordered most to least
// specific.
func readerFor(rc io.ReadCloser, sizeHint int64) (io.ReaderAt, int64) {
	switch v := rc.(type) {
	case *os.File:
		return v, sizeHint
	case byteReaderProvider:
		br := v.ByteReader()
		return br, br.Size()
	case readerAtWithSize:
		return v, v.Size()
	case io.ReaderAt:
		return v, sizeHint
	}
	data, _ := io.ReadAll(rc)
	return bytes.NewReader(data), int64(len(data))
}

// converter replaces a file's bytes with a text rendering of them. Converters
// run before binary and language detection, so detection describes what will
// actually be rendered, and they select on the path rather than the content, so
// choosing one needs no detection. Order matters only if two ever match the
// same path; the first match wins.
type converter struct {
	name    string
	applies func(sources.InputFile) bool
	convert func(path string, r io.ReaderAt, size int64) ([]byte, error)
}

var converters = []converter{
	{
		name: "document",
		applies: func(f sources.InputFile) bool {
			return f.Config.ExtractDocuments && sources.IsDocumentPath(f.Path)
		},
		convert: sources.ExtractDocumentText,
	},
}

// applyConverters reports the format it converted from, or "" if none applied.
// A converter that fails leaves the content untouched.
func applyConverters(file sources.InputFile, reader io.ReaderAt, size int64) (io.ReaderAt, int64, string) {
	for _, c := range converters {
		if !c.applies(file) {
			continue
		}
		text, err := c.convert(file.Path, reader, size)
		if err != nil {
			slog.Debug("Converter failed, keeping raw content",
				"converter", c.name, "path", file.Path, "error", err)
			return reader, size, ""
		}
		return bytes.NewReader(text), int64(len(text)), fileFormat(file.Path)
	}
	return reader, size, ""
}

// detectContent sniffs the header for binary content and a language hint.
func detectContent(path string, reader io.ReaderAt, scratch []byte) (isBinary bool, lang string) {
	headerLen := len(scratch)
	if headerLen > 1024 {
		headerLen = 1024
	}
	n, _ := reader.ReadAt(scratch[:headerLen], 0)
	return internal.IsBinary(scratch[:n]), internal.DetectLanguage(path, scratch[:n])
}

// applySkeleton reduces source to signatures and definitions. Unlike a
// converter it runs after detection, because it needs the detected language. It
// also reports the pre-filter row count, so headers can describe the whole file.
func applySkeleton(cfg core.RunnerConfig, lang string, reader io.ReaderAt, size int64, isBinary, isImage bool) (io.ReaderAt, int64, string, int) {
	if isBinary || isImage || (!cfg.SkeletonFunctions && !cfg.SkeletonTypes) || !skeleton.Supported(lang) {
		return reader, size, "", -1
	}

	allData := make([]byte, size)
	if _, err := reader.ReadAt(allData, 0); err != nil {
		return reader, size, "", -1
	}

	filtered := skeleton.Extract(lang, allData, cfg.SkeletonFunctions, cfg.SkeletonTypes)
	return bytes.NewReader(filtered), int64(len(filtered)),
		skeletonModeLabel(cfg.SkeletonFunctions, cfg.SkeletonTypes), internal.CountLines(allData)
}

// imageDataURI encodes the image from the bytes already open here. Reading them
// back by path in a template would fail for archive entries and URLs, which
// have no readable AbsPath.
func imageDataURI(path string, reader io.ReaderAt, size int64) string {
	if size <= 0 {
		return ""
	}
	data := make([]byte, size)
	if _, err := reader.ReadAt(data, 0); err != nil {
		return ""
	}
	return internal.DataURI(path, data)
}

func fileFormat(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

func skeletonModeLabel(functions, types bool) string {
	switch {
	case functions && types:
		return "definitions"
	case functions:
		return "function signatures"
	default:
		return "type definitions"
	}
}

func (p *Processor) prepareFileContext(file sources.InputFile, index int, scratch []byte) (core.FileContext, error) {
	rc, err := file.Open()
	if err != nil {
		if p.onFileError != nil {
			p.onFileError(file, err)
		}
		return p.errorContext(file, index, err.Error()), nil
	}
	defer rc.Close()

	if f, ok := rc.(*os.File); ok {
		if stat, err := f.Stat(); err == nil && stat.IsDir() {
			return p.errorContext(file, index, "is a directory"), nil
		}
	}

	reader, size := readerFor(rc, file.Size)
	reader, size, convertedFrom := applyConverters(file, reader, size)

	if scratch == nil {
		scratch = make([]byte, 1024)
	}

	isBinary, lang := detectContent(file.Path, reader, scratch)
	isImage := internal.IsImage(file.Path)
	cfg := file.Config

	reader, size, skeletonMode, originalRows := applySkeleton(cfg, lang, reader, size, isBinary, isImage)

	var dataURI string
	if isImage && p.engine.EmbedImages {
		dataURI = imageDataURI(file.Path, reader, size)
	}

	isCompact := (cfg.Head == 0 && cfg.Tail == 0) || isBinary || size == 0
	totalRows, exact, _ := internal.EstimateLineCount(reader, size, scratch)

	// In skeleton mode the header should report the original file's row
	// count, not the filtered output's; slicing still uses totalRows.
	displayRows, displayExact := totalRows, exact
	if originalRows >= 0 {
		displayRows, displayExact = originalRows, true
	}

	var contentData interface{}
	if !isBinary && !isCompact && size > 0 && !isImage {
		effectiveCfg := cfg
		if exact && cfg.Head > 0 && cfg.Tail > 0 && totalRows <= (cfg.Head+cfg.Tail) {
			effectiveCfg.Head = -1
			effectiveCfg.Tail = 0
		}

		head, tail, gap, err := p.readSlices(reader, size, totalRows, !exact, effectiveCfg)
		if err != nil {
			return p.errorContext(file, index, err.Error()), nil
		}
		contentData = p.formatContent(head, tail, gap, totalRows, effectiveCfg)
	}

	return core.FileContext{
		Path:          file.Path,
		AbsPath:       file.AbsPath,
		Size:          size,
		OriginalSize:  file.Size,
		TotalRows:     displayRows,
		IsEstimate:    !displayExact,
		Language:      lang,
		Content:       contentData,
		TokenEstimate: p.tokenCounter(size, contentData),
		IsBinary:      isBinary,
		IsImage:       isImage,
		DataURI:       dataURI,
		IsCompactView: isCompact,
		FileIndex:     index,
		Global:        p.global,
		SkeletonMode:  skeletonMode,
		ConvertedFrom: convertedFrom,
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
	res.Grow(len(head) + len(gap) + len(tail))
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

const (
	maxPreparedBufferHint = 2 << 20 // 2 MiB cap to avoid oversized pre-grows.
	minPreparedBufferHint = 256
)

// EstimatePreparedBufferSize returns a best-effort render output size hint.
func EstimatePreparedBufferSize(item PreparedItem) int {
	switch v := item.Raw.(type) {
	case sources.InputFile:
		hint := minPreparedBufferHint + len(v.Path)
		cfg := v.Config
		switch {
		case cfg.Head == 0 && cfg.Tail == 0:
			// Compact view: header-only output.
		case cfg.Head < 0:
			hint += clampHint(v.Size)
		default:
			// Sliced view: usually much smaller than full file content.
			hint += 16 * 1024
		}
		if cfg.LineNumbers {
			hint += hint / 4
		}
		if hint > maxPreparedBufferHint {
			hint = maxPreparedBufferHint
		}
		return hint
	case core.TreeContext:
		return clampBodyHint(v.Body)
	case core.MetaContext:
		return clampBodyHint(v.Body)
	case core.PromptContext:
		return clampBodyHint(v.Body)
	case core.SectionContext:
		return clampBodyHint(v.Body)
	default:
		return 0
	}
}

func clampHint(size int64) int {
	if size <= 0 {
		return 0
	}
	if size > int64(maxPreparedBufferHint) {
		return maxPreparedBufferHint
	}
	return int(size)
}

func clampBodyHint(body string) int {
	hint := len(body) + minPreparedBufferHint
	if hint < minPreparedBufferHint {
		return minPreparedBufferHint
	}
	if hint > maxPreparedBufferHint {
		return maxPreparedBufferHint
	}
	return hint
}
