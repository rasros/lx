package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/skeleton"
	"github.com/rasros/lx/pkg/lx/sources"
)

// FileItem is a file queued for rendering with the options it renders with.
type FileItem struct {
	File sources.InputFile
	Opts core.RenderOptions
}

// PreparedItem carries precomputed context for deterministic render order.
type PreparedItem struct {
	Raw              interface{}
	Section          *core.SectionContext
	FileIndexGlobal  int
	FileIndexSection int
}

// Processor renders prepared items into an output writer.
type Processor struct {
	engine       *core.TemplateEngine
	global       core.Plan
	tokenCounter core.TokenCounter
}

// ConvertFailure is a converter that failed on a file, which then rendered as
// its raw content.
type ConvertFailure struct {
	Converter string
	Err       error
}

// FileIssues are the problems a file met while rendering, which workers hand
// back for the caller to report in output order.
type FileIssues struct {
	// ReadErr is why a file that rendered as an error row could not be
	// read; it is set exactly when the row is an error.
	ReadErr error
	// Convert is a converter that failed, leaving the raw content.
	Convert *ConvertFailure
}

// RenderStats reports what one rendered item turned out to be. A non-file item
// yields the zero value.
type RenderStats struct {
	Rows      int
	IsCompact bool
	IsBinary  bool
	Issues    FileIssues
}

type readerAtWithSize interface {
	io.ReaderAt
	Size() int64
}

// NewProcessor returns a Processor. It logs nothing: what goes wrong with a
// file comes back in RenderStats.Issues.
func NewProcessor(engine *core.TemplateEngine, global core.Plan) *Processor {
	return &Processor{
		engine:       engine,
		global:       global,
		tokenCounter: core.DefaultTokenCounter,
	}
}

func (p *Processor) SetTokenCounter(tc core.TokenCounter) {
	if tc != nil {
		p.tokenCounter = tc
	}
}

// RenderPrepared processes one item containing pre-calculated section and
// indices. A file whose read fails once ctx is done writes nothing, and
// RenderPrepared returns ctx's error.
func (p *Processor) RenderPrepared(ctx context.Context, w io.Writer, item PreparedItem, scratchBuf []byte) (RenderStats, error) {
	var stats RenderStats
	var data interface{}
	var templateToUse *template.Template

	switch v := item.Raw.(type) {
	case FileItem:
		fCtx, issues := p.prepareFileContext(ctx, v.File, v.Opts, item.FileIndexGlobal, scratchBuf)
		if err := ctx.Err(); err != nil {
			return RenderStats{}, err
		}
		fCtx.Section = *item.Section
		fCtx.SectionFileIndex = item.FileIndexSection

		stats = RenderStats{
			Rows:      fCtx.TotalRows,
			IsCompact: fCtx.IsCompactView,
			IsBinary:  fCtx.IsBinary,
			Issues:    issues,
		}
		data = &fCtx

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

	case core.PromptContext:
		v.Global = p.global
		v.Section = *item.Section
		data = &v
		templateToUse = p.engine.Prompt

	case core.TreeContext:
		v.Global = p.global
		v.Section = *item.Section
		data = &v
		templateToUse = p.engine.Tree

	case core.MetaContext:
		v.Global = p.global
		v.Section = *item.Section
		data = &v
		templateToUse = p.engine.Meta

	default:
		return stats, nil
	}

	if err := templateToUse.Execute(w, data); err != nil {
		return stats, err
	}
	return stats, nil
}

var errIsDir = errors.New("is a directory")

// errorContext describes a file that could not be read.
func (p *Processor) errorContext(file sources.InputFile, index int, msg string) core.FileContext {
	return core.FileContext{
		Path:         file.Path,
		AbsPath:      file.AbsPath,
		Archive:      file.Archive,
		Entry:        file.Entry,
		Size:         file.Size,
		OriginalSize: file.Size,
		ModTime:      file.ModTime,
		FileIndex:    index,
		Global:       p.global,
		ReadError:    msg,
		IsError:      true,
	}
}

// contentLengther is a stream that declares its length up front, as an HTTP
// response body does.
type contentLengther interface {
	ContentLength() int64
}

// readerFor adapts an opened file to random access. A plain stream is read
// lazily, so a converter that needs only headers leaves the rest unread.
func readerFor(rc io.ReadCloser, sizeHint int64) (io.ReaderAt, int64, error) {
	switch v := rc.(type) {
	case *os.File:
		return v, sizeHint, nil
	case readerAtWithSize:
		return v, v.Size(), nil
	case io.ReaderAt:
		if sizeHint >= 0 {
			return v, sizeHint, nil
		}
	}
	declared := int64(-1)
	if cl, ok := rc.(contentLengther); ok {
		declared = cl.ContentLength()
	}
	lr := internal.NewLazyReader(rc, declared)
	return lr, lr.Size(), nil
}

// applyConverters reports whether a converter replaced the content, and the
// format it converted from. Converters run before binary and language
// detection, so detection describes what is rendered; the document converter
// goes first. A failing converter leaves the content untouched and is
// returned. An image about to be embedded is left alone.
func applyConverters(file sources.InputFile, opts core.RenderOptions, format *internal.Format, reader io.ReaderAt, size int64, embedImages bool) (io.ReaderAt, int64, string, string, bool, *ConvertFailure) {
	if embedImages && format.IsImage() {
		return reader, size, "", "", false, nil
	}
	name, lang := "", ""
	var convert func(sources.InputFile, io.ReaderAt, int64) ([]byte, error)
	if opts.ExtractDocuments && sources.IsDocumentInput(file) {
		name, convert = "document", sources.ConvertInput
		// The other formats yield plain text, which detection leaves unlabelled.
		if sources.IsHTMLInput(file) {
			lang = "markdown"
		}
	} else if opts.ExtractMedia && format.IsMedia() {
		name, convert = "media", sources.ExtractMediaMetadata
	} else {
		return reader, size, "", "", false, nil
	}
	text, err := convert(file, reader, size)
	if err != nil {
		return reader, size, "", "", false, &ConvertFailure{Converter: name, Err: err}
	}
	from := fileFormat(file.Path)
	if from == "" && format != nil {
		from = format.Name // an extensionless input, named by its content
	}
	return bytes.NewReader(text), int64(len(text)), from, lang, true, nil
}

// classify decides the file's format from its content before any converter
// sees it. It also reports whether the bytes verify the format; only a
// verified image renders as a picture.
func classify(path string, reader io.ReaderAt, scratch []byte) (*internal.Format, bool) {
	n, _ := reader.ReadAt(scratch[:min(len(scratch), internal.SniffLen)], 0)
	format := internal.Classify(path, scratch[:n])
	return format, format.Verify(scratch[:n])
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

// skeletonView is a file reduced to its skeleton: the lines kept from the
// source, each with the source row it came from.
type skeletonView struct {
	lines internal.KeptLines
	rows  int    // rows in the whole source, which the header reports
	mode  string // what the skeleton shows, for the header
}

// applySkeleton reduces source to signatures and definitions, or returns nil
// when the file is not reduced. It runs after detection, which picks the
// language.
func applySkeleton(cfg core.RenderOptions, lang string, header bool, reader io.ReaderAt, size int64, isBinary, isImage bool) *skeletonView {
	if isBinary || isImage || (!cfg.SkeletonFunctions && !cfg.SkeletonTypes) || !skeleton.Supported(lang) {
		return nil
	}

	allData, err := io.ReadAll(io.NewSectionReader(reader, 0, size))
	if err != nil {
		return nil
	}

	return &skeletonView{
		lines: skeleton.Keep(lang, header, allData, cfg.SkeletonFunctions, cfg.SkeletonTypes),
		rows:  internal.CountLines(allData),
		mode:  skeletonModeLabel(cfg.SkeletonFunctions, cfg.SkeletonTypes),
	}
}

// skeletonSlices cuts what cfg asks for from a skeleton. Head and tail count
// skeleton lines, while line numbers and the gap count source rows.
func skeletonSlices(sk *skeletonView, cfg core.RenderOptions) excerpt {
	n := sk.lines.Len()
	sl := excerpt{rows: sk.rows, exact: true}
	head, tail := cfg.Head, max(cfg.Tail, 0)
	if head < 0 || head+tail >= n {
		sl.head, sl.headRows = sk.lines.Render(0, n)
		return sl
	}
	sl.head, sl.headRows = sk.lines.Render(0, head)
	sl.tail, sl.tailRows = sk.lines.Render(n-tail, n)
	if head > 0 && tail > 0 {
		skipped := sk.lines.Rows[n-tail] - sk.lines.Rows[head-1] - 1
		sl.gap = []byte(fmt.Sprintf("... (%d rows skipped)\n", skipped))
	}
	return sl
}

// imageDataURI encodes the image from the open bytes, since archive entries
// and URLs have no AbsPath a template could read.
func imageDataURI(format *internal.Format, reader io.ReaderAt, size int64) string {
	if size <= 0 {
		return ""
	}
	data, err := io.ReadAll(io.NewSectionReader(reader, 0, size))
	if err != nil {
		return ""
	}
	return internal.DataURIWithType(format.MIME, data)
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

// prepareFileContext describes file for its template. A file that cannot be
// read becomes an error row. Once ctx is done the result is to be discarded.
func (p *Processor) prepareFileContext(ctx context.Context, file sources.InputFile, cfg core.RenderOptions, index int, scratch []byte) (core.FileContext, FileIssues) {
	var issues FileIssues
	rc, err := file.Open(ctx)
	if err != nil {
		issues.ReadErr = err
		return p.errorContext(file, index, err.Error()), issues
	}
	defer rc.Close()

	if f, ok := rc.(*os.File); ok {
		if stat, err := f.Stat(); err == nil && stat.IsDir() {
			issues.ReadErr = errIsDir
			return p.errorContext(file, index, errIsDir.Error()), issues
		}
	}

	// A stream that may never end reads up to the size limit, which for
	// files that state their size is applied before they are opened.
	var reader io.ReaderAt
	var size int64
	if stream, ok := rc.(*sources.Stream); ok {
		var data []byte
		data, err = sources.ReadStream(stream, cfg.MaxSize)
		reader, size = bytes.NewReader(data), int64(len(data))
	} else {
		reader, size, err = readerFor(rc, file.Size)
	}
	if err != nil {
		issues.ReadErr = err
		return p.errorContext(file, index, err.Error()), issues
	}

	if scratch == nil {
		scratch = make([]byte, 1024)
	}

	format, verified := classify(file.Path, reader, scratch)
	reader, size, convertedFrom, convertedLang, converted, convertFailure := applyConverters(
		file, cfg, format, reader, size, p.engine.EmbedImages && verified)
	issues.Convert = convertFailure

	isBinary, lang := detectContent(file.Path, reader, scratch)
	if convertedLang != "" {
		lang = convertedLang
	}
	// A converted image renders as text.
	isImage := format.IsImage() && verified && !converted

	skel := applySkeleton(cfg, lang, strings.EqualFold(filepath.Ext(file.Path), ".h"), reader, size, isBinary, isImage)
	var skeletonMode string
	if skel != nil {
		skeletonMode = skel.mode
		size = int64(skel.lines.Size())
	}

	var dataURI string
	if isImage && p.engine.EmbedImages {
		dataURI = imageDataURI(format, reader, size)
	}

	isCompact := (cfg.Head == 0 && cfg.Tail == 0) || isBinary || size == 0

	var (
		content      interface{}
		totalRows    int
		rowsAreExact = true
	)
	switch {
	case skel != nil:
		totalRows = skel.rows
		if !isCompact {
			content = formatContent(skeletonSlices(skel, cfg), cfg.LineNumbers)
		}
	case !isBinary && !isImage:
		lines := internal.NewLineSource(reader, size)
		var sl excerpt
		if isCompact {
			sl.rows, sl.exact, err = lines.Count()
		} else {
			sl, err = readSlices(lines, cfg)
		}
		if err != nil {
			issues.ReadErr = err
			return p.errorContext(file, index, err.Error()), issues
		}
		size = lines.Size()
		totalRows, rowsAreExact = sl.rows, sl.exact
		if !isCompact {
			content = formatContent(sl, cfg.LineNumbers)
		}
	}

	fCtx := core.FileContext{
		Path:          file.Path,
		AbsPath:       file.AbsPath,
		Archive:       file.Archive,
		Entry:         file.Entry,
		Size:          size,
		OriginalSize:  file.Size,
		TotalRows:     totalRows,
		IsEstimate:    !rowsAreExact,
		Language:      lang,
		Content:       content,
		IsBinary:      isBinary,
		IsImage:       isImage,
		DataURI:       dataURI,
		IsCompactView: isCompact,
		FileIndex:     index,
		Global:        p.global,
		SkeletonMode:  skeletonMode,
		ConvertedFrom: convertedFrom,
	}
	core.SetTokenCounter(&fCtx, p.tokenCounter)
	return fCtx, issues
}

// excerpt is the part of a file that gets rendered: a head, and a tail with a
// gap marker between them when lines in the middle are left out.
type excerpt struct {
	head, gap, tail []byte
	rows            int  // lines in the whole file
	exact           bool // whether rows is a count rather than an estimate

	// headRows and tailRows number the lines of head and tail when they are
	// not consecutive rows, as in a skeleton; nil numbers them in sequence.
	headRows, tailRows []int
}

// readSlices reads what cfg asks for from lines. A head and tail that meet
// render the whole file.
func readSlices(lines *internal.LineSource, cfg core.RenderOptions) (excerpt, error) {
	if cfg.Head < 0 {
		all, err := lines.All()
		return excerpt{head: terminate(all), rows: internal.CountLines(all), exact: true}, err
	}

	head, err := lines.Head(cfg.Head)
	if err != nil {
		return excerpt{}, err
	}
	tail, err := lines.Tail(cfg.Tail)
	if err != nil {
		return excerpt{}, err
	}

	if overlap := int64(len(head)+len(tail)) - lines.Size(); overlap >= 0 {
		all := append(head[:len(head):len(head)], tail[overlap:]...)
		return excerpt{head: terminate(all), rows: internal.CountLines(all), exact: true}, nil
	}

	rows, exact, err := lines.Count()
	if err != nil {
		return excerpt{}, err
	}
	// Something lies between head and tail, even if the estimate says not.
	shown := internal.CountLines(head) + internal.CountLines(tail)
	rows = max(rows, shown+1)
	sl := excerpt{head: head, tail: tail, rows: rows, exact: exact}
	if len(head) > 0 && len(tail) > 0 {
		tilde := ""
		if !exact {
			tilde = "~"
		}
		sl.gap = []byte(fmt.Sprintf("... (%s%d rows skipped)\n", tilde, rows-shown))
	}
	return sl, nil
}

// terminate ends a head with a newline when its last line has none.
func terminate(head []byte) []byte {
	if len(head) == 0 || head[len(head)-1] == '\n' {
		return head
	}
	return append(head[:len(head):len(head)], '\n')
}

// formatContent joins the slices into the text the templates render, numbering
// the lines when asked.
func formatContent(sl excerpt, lineNumbers bool) string {
	if lineNumbers {
		return internal.LineNumberFormatter{
			Head:        sl.head,
			Gap:         sl.gap,
			Tail:        sl.tail,
			HeadRows:    sl.headRows,
			TailRows:    sl.tailRows,
			TotalRows:   sl.rows,
			Approximate: !sl.exact,
		}.String()
	}
	var res strings.Builder
	res.Grow(len(sl.head) + len(sl.gap) + len(sl.tail))
	res.Write(sl.head)
	res.Write(sl.gap)
	res.Write(sl.tail)
	return res.String()
}

const (
	maxPreparedBufferHint = 2 << 20 // 2 MiB cap to avoid oversized pre-grows.
	minPreparedBufferHint = 256
)

// EstimatePreparedBufferSize returns a best-effort render output size hint.
func EstimatePreparedBufferSize(item PreparedItem) int {
	switch v := item.Raw.(type) {
	case FileItem:
		hint := minPreparedBufferHint + len(v.File.Path)
		cfg := v.Opts
		switch {
		case cfg.Head == 0 && cfg.Tail == 0:
			// Compact view: header-only output.
		case cfg.Head < 0:
			hint += clampHint(v.File.Size)
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
