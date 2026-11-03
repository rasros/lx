package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

// A file whose read a cancel cut short renders nothing, not an error row
// about the cancel.
func TestRender_CancelledReadWritesNothing(t *testing.T) {
	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.Plan{})

	ctx, cancel := context.WithCancel(context.Background())
	file := sources.InputFile{Path: "slow.txt", Size: -1, Open: func(ctx context.Context) (io.ReadCloser, error) {
		cancel() // the run is cancelled while the file is being read
		return io.NopCloser(iotest.ErrReader(ctx.Err())), nil
	}}
	item := PreparedItem{Raw: FileItem{File: file, Opts: core.RenderOptions{Head: -1}}, Section: &core.SectionContext{}}

	var buf bytes.Buffer
	_, err := proc.RenderPrepared(ctx, &buf, item, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RenderPrepared error = %v, want context.Canceled", err)
	}
	if buf.Len() != 0 {
		t.Errorf("cancelled file rendered:\n%s", buf.String())
	}
}

func TestSkeletonModeLabel(t *testing.T) {
	cases := []struct {
		functions, types bool
		want             string
	}{
		{true, true, "definitions"},
		{true, false, "function signatures"},
		{false, true, "type definitions"},
	}
	for _, c := range cases {
		if got := skeletonModeLabel(c.functions, c.types); got != c.want {
			t.Errorf("skeletonModeLabel(%v, %v) = %q, want %q", c.functions, c.types, got, c.want)
		}
	}
}

func TestReaderForBuffersNonSeekableReaders(t *testing.T) {
	// A plain ReadCloser exposes neither ReaderAt nor Size, so it must be read
	// into memory and reported at its true length rather than the hint.
	rc := io.NopCloser(strings.NewReader("hello world"))
	reader, size, err := readerFor(rc, 999)
	if err != nil {
		t.Fatal(err)
	}

	if size != 11 {
		t.Errorf("size = %d, want 11", size)
	}
	buf := make([]byte, size)
	if _, err := reader.ReadAt(buf, 0); err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if string(buf) != "hello world" {
		t.Errorf("content = %q", buf)
	}
}

func TestFileFormat(t *testing.T) {
	cases := map[string]string{
		"a/b/manual.PDF": "pdf",
		"notes.docx":     "docx",
		"Makefile":       "",
	}
	for path, want := range cases {
		if got := fileFormat(path); got != want {
			t.Errorf("fileFormat(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestApplyConvertersLeavesContentWhenNoneApply(t *testing.T) {
	src := []byte("package main\n")
	file := sources.NewBufferInputFile("main.go", src)

	reader, size, from, _, _, _ := applyConverters(file, core.RenderOptions{}, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
	if from != "" {
		t.Errorf("convertedFrom = %q, want empty", from)
	}
	if size != int64(len(src)) {
		t.Errorf("size = %d, want %d", size, len(src))
	}
	got := make([]byte, size)
	reader.ReadAt(got, 0)
	if string(got) != string(src) {
		t.Errorf("content = %q, want it untouched", got)
	}
}

func TestApplyConvertersRunsMatchingConverter(t *testing.T) {
	converters = append(converters, converter{
		name: "test",
		applies: func(f sources.InputFile, _ core.RenderOptions, _ *internal.Format) bool {
			return strings.HasSuffix(f.Path, ".zzz")
		},
		convert: func(f sources.InputFile, r io.ReaderAt, size int64) ([]byte, error) {
			return []byte("converted"), nil
		},
	})
	t.Cleanup(func() { converters = converters[:len(converters)-1] })

	file := sources.NewBufferInputFile("doc.zzz", []byte("raw"))
	reader, size, from, _, _, _ := applyConverters(file, core.RenderOptions{}, nil, bytes.NewReader([]byte("raw")), 3, false)

	if from != "zzz" {
		t.Errorf("convertedFrom = %q, want %q", from, "zzz")
	}
	got := make([]byte, size)
	reader.ReadAt(got, 0)
	if string(got) != "converted" {
		t.Errorf("content = %q, want %q", got, "converted")
	}
}

// A failing converter must not lose the original content.
func TestApplyConvertersKeepsContentOnFailure(t *testing.T) {
	converters = append(converters, converter{
		name: "failing",
		applies: func(f sources.InputFile, _ core.RenderOptions, _ *internal.Format) bool {
			return strings.HasSuffix(f.Path, ".zzz")
		},
		convert: func(f sources.InputFile, r io.ReaderAt, size int64) ([]byte, error) {
			return nil, io.ErrUnexpectedEOF
		},
	})
	t.Cleanup(func() { converters = converters[:len(converters)-1] })

	src := []byte("raw content")
	reader, size, from, _, _, failure := applyConverters(sources.NewBufferInputFile("doc.zzz", src), core.RenderOptions{}, nil, bytes.NewReader(src), int64(len(src)), false)

	if failure == nil || failure.Converter != "failing" || failure.Err != io.ErrUnexpectedEOF {
		t.Errorf("failure = %v, want the failing converter's error", failure)
	}
	if from != "" {
		t.Errorf("convertedFrom = %q, want empty after a failure", from)
	}
	got := make([]byte, size)
	reader.ReadAt(got, 0)
	if string(got) != string(src) {
		t.Errorf("content = %q, want the original %q", got, src)
	}
}

func TestApplySkeletonReportsOriginalRowCount(t *testing.T) {
	src := []byte("package main\n\nfunc Alpha() int {\n\treturn 1\n}\n")
	cfg := core.RenderOptions{Head: -1, SkeletonFunctions: true}

	sk := applySkeleton(cfg, "go", false, bytes.NewReader(src), int64(len(src)), false, false)
	if sk == nil {
		t.Fatal("skeleton did not run")
	}
	if sk.mode != "function signatures" {
		t.Errorf("mode = %q", sk.mode)
	}
	if sk.rows != 5 {
		t.Errorf("rows = %d, want 5", sk.rows)
	}
	if got := string(sk.lines.Text()); got != "func Alpha() int {\n" {
		t.Errorf("text = %q", got)
	}
	if want := []int{3}; !slices.Equal(sk.lines.Rows, want) {
		t.Errorf("rows = %v, want %v", sk.lines.Rows, want)
	}
}

func TestApplySkeletonSkipsBinaryAndImages(t *testing.T) {
	src := []byte("package main\nfunc A() {}\n")
	cfg := core.RenderOptions{Head: -1, SkeletonFunctions: true}

	for _, tc := range []struct {
		name              string
		isBinary, isImage bool
	}{
		{"binary", true, false},
		{"image", false, true},
	} {
		if sk := applySkeleton(cfg, "go", false, bytes.NewReader(src), int64(len(src)), tc.isBinary, tc.isImage); sk != nil {
			t.Errorf("%s: skeleton ran anyway (mode=%q)", tc.name, sk.mode)
		}
	}
}

// Non-file items carry no stats, and must not inherit them from a prior item.
func TestRenderPreparedNonFileItemHasZeroStats(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.Plan{})

	file := sources.NewBufferInputFile("a.txt", []byte("one\ntwo\n"))
	opts := core.RenderOptions{Head: -1}
	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(context.Background(), &buf, PreparedItem{
		Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1,
	}, nil); err != nil {
		t.Fatal(err)
	}

	stats, err := proc.RenderPrepared(context.Background(), &buf, PreparedItem{
		Raw: core.PromptContext{Body: "hi"}, Section: &core.SectionContext{},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (RenderStats{}) {
		t.Errorf("prompt item reported %+v, want zero stats", stats)
	}
}

func TestApplyConvertersReportsProducedLanguage(t *testing.T) {
	src := []byte("<h1>Hi</h1>")
	file := sources.NewBufferInputFile("page.html", src)
	opts := core.RenderOptions{Head: -1, ExtractDocuments: true}

	_, _, from, lang, _, _ := applyConverters(file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
	if from != "html" {
		t.Errorf("convertedFrom = %q, want html", from)
	}
	if lang != "markdown" {
		t.Errorf("language = %q, want markdown", lang)
	}
}

// An image bound for embedding keeps its bytes: the picture carries more than a
// description of it would.
func TestApplyConvertersLeavesImagesBeingEmbedded(t *testing.T) {
	src := []byte("\x89PNG\r\n\x1a\n")
	file := sources.NewBufferInputFile("logo.png", src)
	opts := core.RenderOptions{Head: -1, ExtractMedia: true}

	if _, _, from, _, _, _ := applyConverters(file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), true); from != "" {
		t.Errorf("convertedFrom = %q, want empty while embedding", from)
	}
	if _, _, from, _, _, _ := applyConverters(file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false); from != "png" {
		t.Errorf("convertedFrom = %q, want png when not embedding", from)
	}
}

func TestApplyConvertersReportsNoLanguageWhenNoneApply(t *testing.T) {
	src := []byte("package main\n")
	file := sources.NewBufferInputFile("main.go", src)

	_, _, from, lang, _, _ := applyConverters(file, core.RenderOptions{}, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
	if from != "" || lang != "" {
		t.Errorf("got from=%q lang=%q, want both empty", from, lang)
	}
}

// bigLines is a large text file with every line distinct so its position can
// be checked.
func bigLines(n int) []byte {
	var b bytes.Buffer
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %05d %s\n", i, strings.Repeat("-", 20))
	}
	return b.Bytes()
}

// Tail line numbers come from the exact row count, whichever kind of reader
// the file arrives through: an *os.File, a *bytes.Reader, or any other
// io.ReaderAt, such as a file from an fs.FS.
func TestPrepareFileContextNumbersBigTailExactly(t *testing.T) {
	data := bigLines(3000)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	osFile, err := sources.NewInputFileFromPath(os.DirFS(dir), "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	mapFile, err := sources.NewInputFileFromPath(fstest.MapFS{"big.txt": {Data: data}}, "big.txt")
	if err != nil {
		t.Fatal(err)
	}

	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.Plan{})
	pad := strings.Repeat("-", 20)
	want := "   1: line 00001 " + pad + "\n" +
		"   2: line 00002 " + pad + "\n" +
		"... (2995 rows skipped)\n" +
		"2998: line 02998 " + pad + "\n" +
		"2999: line 02999 " + pad + "\n" +
		"3000: line 03000 " + pad + "\n"

	for name, file := range map[string]sources.InputFile{
		"os.File":      osFile,
		"bytes.Reader": sources.NewBufferInputFile("big.txt", data),
		"fstest.MapFS": mapFile,
	} {
		opts := core.RenderOptions{Head: 2, Tail: 3, LineNumbers: true}
		ctx, issues := proc.prepareFileContext(context.Background(), file, opts, 1, nil)
		if issues.ReadErr != nil {
			t.Fatal(issues.ReadErr)
		}
		if got, _ := ctx.Content.(string); got != want {
			t.Errorf("%s: content =\n%s\nwant\n%s", name, got, want)
		}
		if ctx.TotalRows != 3000 {
			t.Errorf("%s: rows=%d, want 3000", name, ctx.TotalRows)
		}
	}
}

// hugeFile serves a file too large to count exactly without allocating it: a
// first line longer than the chunk the row estimate is drawn from, then short
// lines to the end. The estimate from that chunk comes out far too low.
type hugeFile struct{ size, firstLine int64 }

func (h hugeFile) Size() int64  { return h.size }
func (h hugeFile) Close() error { return nil }
func (h hugeFile) Read([]byte) (int, error) {
	return 0, io.EOF // only ever read through ReadAt
}
func (h hugeFile) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for ; n < len(p) && off+int64(n) < h.size; n++ {
		pos := off + int64(n)
		switch {
		case pos < h.firstLine:
			p[n] = 'x'
		case pos == h.firstLine, (pos-h.firstLine)%4 == 0:
			p[n] = '\n'
		default:
			p[n] = 'r'
		}
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Beyond internal.ExactCountLimit the row count is an estimate, which must
// still leave room for the lines shown and the gap known to lie between them.
func TestPrepareFileContextClampsEstimatedRows(t *testing.T) {
	h := hugeFile{size: internal.ExactCountLimit + 80<<20, firstLine: 100 << 10}
	file := sources.InputFile{
		Path: "huge.log",
		Size: h.size,
		Open: func(context.Context) (io.ReadCloser, error) { return h, nil },
	}
	opts := core.RenderOptions{Head: 2, Tail: 2, LineNumbers: true}

	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.Plan{})
	ctx, issues := proc.prepareFileContext(context.Background(), file, opts, 1, nil)
	if issues.ReadErr != nil {
		t.Fatal(issues.ReadErr)
	}
	if !ctx.IsEstimate {
		t.Fatal("want an estimated row count for a file beyond ExactCountLimit")
	}
	if ctx.TotalRows < 5 {
		t.Errorf("rows = %d, want at least the 4 lines shown plus 1 skipped", ctx.TotalRows)
	}
	content, _ := ctx.Content.(string)
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[2], "... (~") {
		t.Fatalf("want head, gap marker and tail, got %d lines; gap line %q", len(lines), lines[min(2, len(lines)-1)])
	}
	width := strings.Index(lines[0], ":")
	for _, i := range []int{3, 4} {
		if !strings.HasPrefix(strings.TrimLeft(lines[i], " "), "~") || strings.Index(lines[i], ":") != width {
			t.Errorf("tail line %q, want a ~ number aligned with the head's", lines[i][:min(len(lines[i]), 20)])
		}
	}
	tailFirst, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[3][:width]), "~")))
	if tailFirst <= 2 {
		t.Errorf("tail starts at row %d, want after the head's rows 1-2", tailFirst)
	}
}
