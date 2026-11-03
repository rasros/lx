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
	StreamIndex      int
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

type byteReaderProvider interface {
	ByteReader() *bytes.Reader
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
// indices. Once ctx is done it stops reading and converting the file, writes
// nothing and returns ctx's error.
func (p *Processor) RenderPrepared(ctx context.Context, w io.Writer, item PreparedItem, scratchBuf []byte) (RenderStats, error) {
	var stats RenderStats
	var data interface{}
	var templateToUse *template.Template

	switch v := item.Raw.(type) {
	case FileItem:
		fCtx, issues := p.prepareFileContext(ctx, v.File, v.Opts, item.FileIndexGlobal, scratchBuf)
		// What a cancelled file made of its failed reads is no description
		// of it.
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

// readerFor adapts an opened file to random access. Cases are ordered most to
// least specific. A stream that offers neither is read lazily, as far as the
// consumers ask: a converter that needs only headers leaves the rest unread.
// Its size is the length it declares, or else what reading it to the end finds.
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
		if sizeHint >= 0 {
			return v, sizeHint
		}
	}
	declared := int64(-1)
	if cl, ok := rc.(contentLengther); ok {
		declared = cl.ContentLength()
	}
	lr := internal.NewLazyReader(rc, declared)
	return lr, lr.Size()
}

// converter replaces a file's bytes with a text rendering of them. Converters
// run before binary and language detection, so detection describes what will
// actually be rendered. They select on the path and on the format the file was
// classified as, which is decided from its content before any converter runs.
// Order matters only if two ever match the same file; the first match wins.
type converter struct {
	name    string
	applies func(f sources.InputFile, opts core.RenderOptions, format *internal.Format) bool
	convert func(ctx context.Context, f sources.InputFile, r io.ReaderAt, size int64) ([]byte, error)
	// language of the produced content; empty means detect as usual.
	language func(f sources.InputFile) string
}

var converters = []converter{
	{
		name: "document",
		applies: func(f sources.InputFile, opts core.RenderOptions, _ *internal.Format) bool {
			if !opts.ExtractDocuments {
				return false
			}
			return sources.IsDocumentInput(f)
		},
		convert: sources.ConvertInput,
		language: func(f sources.InputFile) string {
			// The other formats yield plain text, which detection leaves unlabelled.
			if sources.IsHTMLInput(f) {
				return "markdown"
			}
			return ""
		},
	},
	{
		name: "media",
		applies: func(_ sources.InputFile, opts core.RenderOptions, format *internal.Format) bool {
			return opts.ExtractMedia && format.IsMedia()
		},
		// Media is described from its headers, read through the opened
		// source, whose reads already stop with ctx.
		convert: func(_ context.Context, f sources.InputFile, r io.ReaderAt, size int64) ([]byte, error) {
			return sources.ExtractMediaMetadata(f, r, size)
		},
	},
}

// applyConverters reports whether a converter replaced the content, and the
// format it converted from. A converter that fails leaves the content untouched
// and is returned.
//
// An image that is about to be embedded is left alone: the picture itself says
// more than a description of it would.
func applyConverters(ctx context.Context, file sources.InputFile, opts core.RenderOptions, format *internal.Format, reader io.ReaderAt, size int64, embedImages bool) (io.ReaderAt, int64, string, string, bool, *ConvertFailure) {
	if embedImages && format.IsImage() {
		return reader, size, "", "", false, nil
	}
	for _, c := range converters {
		if !c.applies(file, opts, format) {
			continue
		}
		text, err := c.convert(ctx, file, reader, size)
		if err != nil {
			return reader, size, "", "", false, &ConvertFailure{Converter: c.name, Err: err}
		}
		lang := ""
		if c.language != nil {
			lang = c.language(file)
		}
		from := fileFormat(file.Path)
		if from == "" && format != nil {
			from = format.Name // an extensionless input, named by its content
		}
		return bytes.NewReader(text), int64(len(text)), from, lang, true, nil
	}
	return reader, size, "", "", false, nil
}

// classify reads the start of the file and decides its format before any
// converter sees it, so that the content rather than the name has the last word.
// It also reports whether the bytes bear the format out, which a name or a weak
// signature alone does not; only such a file is rendered as a picture.
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
// when the file is not reduced. Unlike a converter it runs after detection,
// because it needs the detected language.
func applySkeleton(cfg core.RenderOptions, lang string, reader io.ReaderAt, size int64, isBinary, isImage bool) *skeletonView {
	if isBinary || isImage || (!cfg.SkeletonFunctions && !cfg.SkeletonTypes) || !skeleton.Supported(lang) {
		return nil
	}

	allData, err := internal.ReadAllAt(reader, size)
	if err != nil {
		return nil
	}

	return &skeletonView{
		lines: skeleton.Keep(lang, allData, cfg.SkeletonFunctions, cfg.SkeletonTypes),
		rows:  internal.CountLines(allData),
		mode:  skeletonModeLabel(cfg.SkeletonFunctions, cfg.SkeletonTypes),
	}
}

// skeletonSlices cuts what cfg asks for from a skeleton. Head and tail count
// skeleton lines, as they count the lines of any other content, but each line
// keeps its source row, and the gap between them counts the source rows it
// leaves out, the same rows the header and line numbers count.
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

// imageDataURI encodes the image from the bytes already open here. Reading them
// back by path in a template would fail for archive entries and URLs, which
// have no readable AbsPath.
func imageDataURI(format *internal.Format, reader io.ReaderAt, size int64) string {
	if size <= 0 {
		return ""
	}
	data, err := internal.ReadAllAt(reader, size)
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
// read becomes an error row; that and a failed converter are returned for
// the caller to report. Once ctx is done the result describes nothing and is
// to be discarded.
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

	reader, size := readerFor(rc, file.Size)

	if scratch == nil {
		scratch = make([]byte, 1024)
	}

	format, verified := classify(file.Path, reader, scratch)
	reader, size, convertedFrom, convertedLang, converted, convertFailure := applyConverters(
		ctx, file, cfg, format, reader, size, p.engine.EmbedImages && verified)
	issues.Convert = convertFailure

	isBinary, lang := detectContent(file.Path, reader, scratch)
	if convertedLang != "" {
		lang = convertedLang
	}
	// A converted image is no longer an image but the text describing it, which
	// the content templates render and the image template would discard.
	isImage := format.IsImage() && verified && !converted

	// A skeleton is parsed from bytes already read, which ctx cannot
	// interrupt, so a cancelled file stops before it.
	if ctx.Err() != nil {
		return core.FileContext{}, issues
	}
	skel := applySkeleton(cfg, lang, reader, size, isBinary, isImage)
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
		// The header reports the whole source's rows, which line numbers and
		// gaps count too.
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

// readSlices reads what cfg asks for from lines. A head and tail that meet or
// overlap cover the whole file, which then renders whole, and any read that
// covers the whole file yields an exact row count.
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

	// Head and tail are prefix and suffix of the content, so their byte
	// lengths alone tell whether anything lies between them.
	if overlap := int64(len(head)+len(tail)) - lines.Size(); overlap >= 0 {
		all := append(head[:len(head):len(head)], tail[overlap:]...)
		return excerpt{head: terminate(all), rows: internal.CountLines(all), exact: true}, nil
	}

	rows, exact, err := lines.Count()
	if err != nil {
		return excerpt{}, err
	}
	// Bytes lie between head and tail, so at least one line does too. An
	// estimate from the start of the file can fall short of that.
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

// terminate ends a head with a newline when its last line has none, as the
// head always has been, so templates that print Content as-is keep their
// output.
func terminate(head []byte) []byte {
	if len(head) == 0 || head[len(head)-1] == '\n' {
		return head
	}
	return append(head[:len(head):len(head)], '\n')
}

// formatContent joins the slices into the text the templates render, numbering
// the lines when asked. The text is built once here, so templates and the token
// estimate all read the same string.
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
