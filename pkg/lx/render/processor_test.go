package render

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

func TestProcessor_RenderFile_Slicing(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	global := core.GlobalContext{TotalFiles: 1}

	proc := NewProcessor(engine, global, nil)

	file := sources.NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))
	opts := core.RenderOptions{Head: 1, Tail: 1}

	scratch := make([]byte, 1024)
	item := PreparedItem{Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, scratch); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "1\n") || !strings.Contains(got, "5\n") {
		t.Error("Output missing head or tail")
	}
	if !strings.Contains(got, "rows skipped") {
		t.Error("Output missing gap marker")
	}
}

// skeletonThreeFuncs keeps rows 3, 7 and 11 in a function skeleton.
const skeletonThreeFuncs = `package p

func A() {
	println(1)
}

func B() {
	println(2)
}

func C() {
	println(3)
}
`

func renderSkeleton(t *testing.T, rc core.RenderOptions) string {
	t.Helper()
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{TotalFiles: 1}, nil)

	file := sources.NewBufferInputFile("skeleton.go", []byte(skeletonThreeFuncs))
	item := PreparedItem{Raw: FileItem{File: file, Opts: rc}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestProcessor_RenderFile_SkeletonSlicingKeepsSourceRows(t *testing.T) {
	got := renderSkeleton(t, core.RenderOptions{Head: 1, Tail: 1, SkeletonFunctions: true})
	if !strings.Contains(got, "(13 rows, function signatures)") {
		t.Fatalf("expected original row count in header, got:\n%s", got)
	}
	// Head keeps A (row 3), tail keeps C (row 11): rows 4-10 lie between.
	if !strings.Contains(got, "func A() {\n... (7 rows skipped)\nfunc C() {\n") {
		t.Fatalf("expected a gap counted in source rows, got:\n%s", got)
	}
}

func TestProcessor_RenderFile_SkeletonNumbersSourceRows(t *testing.T) {
	got := renderSkeleton(t, core.RenderOptions{Head: -1, SkeletonFunctions: true, LineNumbers: true})
	want := " 3: func A() {\n...\n 7: func B() {\n...\n11: func C() {\n"
	if !strings.Contains(got, want) {
		t.Fatalf("want source row numbers and elision markers\n%s\ngot:\n%s", want, got)
	}

	got = renderSkeleton(t, core.RenderOptions{Head: 1, Tail: 1, SkeletonFunctions: true, LineNumbers: true})
	want = " 3: func A() {\n... (7 rows skipped)\n11: func C() {\n"
	if !strings.Contains(got, want) {
		t.Fatalf("want sliced lines to keep source rows\n%s\ngot:\n%s", want, got)
	}

	got = renderSkeleton(t, core.RenderOptions{Head: 0, Tail: 2, SkeletonFunctions: true, LineNumbers: true})
	want = " 7: func B() {\n...\n11: func C() {\n"
	if !strings.Contains(got, want) {
		t.Fatalf("want tail numbered by source rows, not counted back\n%s\ngot:\n%s", want, got)
	}
}

func TestRender_DataURI(t *testing.T) {
	tmp := t.TempDir()
	imgName := "test.png"
	imgPath := filepath.Join(tmp, imgName)
	payload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(imgPath, payload, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.NewConfig()
	cfg.OutputFormat = "html"
	engine, _ := templatex.Compile(cfg)

	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file, err := sources.NewInputFileFromPath(os.DirFS(tmp), imgName)
	if err != nil {
		t.Fatal(err)
	}
	file.AbsPath = imgPath

	item := PreparedItem{Raw: FileItem{File: file}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "<img src=\"data:image/png;base64,") {
		t.Errorf("HTML output did not contain data URI. Got:\n%s", got)
	}
}

func TestRender_ErrorHandling(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)

	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.InputFile{Path: "ghost.txt", Open: func() (io.ReadCloser, error) { return nil, os.ErrPermission }}
	item := PreparedItem{Raw: FileItem{File: file}, Section: &core.SectionContext{}}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "error:") {
		t.Errorf("Expected error template output, got: %s", got)
	}
}

func TestRender_BinaryFile(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	binaryContent := append([]byte("ELF"), 0x00, 0x01, 0x02, 0x03)
	file := sources.NewBufferInputFile("program", binaryContent)
	opts := core.RenderOptions{Head: -1, Tail: -1}

	item := PreparedItem{Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "binary file skipped") {
		t.Errorf("Expected binary file message, got:\n%s", got)
	}
}

func TestRender_CompactView(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("data.txt", []byte("lots of content\n"))
	opts := core.RenderOptions{Head: 0, Tail: 0}

	item := PreparedItem{Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "data.txt") {
		t.Errorf("Expected filename in compact output, got:\n%s", got)
	}
	if strings.Contains(got, "lots of content") {
		t.Errorf("Compact view should not include file content, got:\n%s", got)
	}
}

func TestRender_LineNumbers(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{TotalFiles: 1}, nil)

	file := sources.NewBufferInputFile("code.go", []byte("package main\nfunc main() {}\n"))
	opts := core.RenderOptions{Head: -1, Tail: -1, LineNumbers: true}

	item := PreparedItem{Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "1: package main") {
		t.Errorf("Expected line numbers in output, got:\n%s", got)
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
	reader, size := readerFor(rc, 999)

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

	reader, size, from, _, _ := applyConverters(slog.Default(), file, core.RenderOptions{}, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
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
	reader, size, from, _, _ := applyConverters(slog.Default(), file, core.RenderOptions{}, nil, bytes.NewReader([]byte("raw")), 3, false)

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
	reader, size, from, _, _ := applyConverters(slog.Default(),
		sources.NewBufferInputFile("doc.zzz", src), core.RenderOptions{}, nil, bytes.NewReader(src), int64(len(src)), false)

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

	sk := applySkeleton(cfg, "go", bytes.NewReader(src), int64(len(src)), false, false)
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
		if sk := applySkeleton(cfg, "go", bytes.NewReader(src), int64(len(src)), tc.isBinary, tc.isImage); sk != nil {
			t.Errorf("%s: skeleton ran anyway (mode=%q)", tc.name, sk.mode)
		}
	}
}

func TestRenderPreparedReportsStats(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("a.txt", []byte("one\ntwo\nthree\n"))
	opts := core.RenderOptions{Head: -1}

	var buf bytes.Buffer
	stats, err := proc.RenderPrepared(&buf, PreparedItem{
		Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rows != 3 {
		t.Errorf("Rows = %d, want 3", stats.Rows)
	}
	if stats.IsBinary || stats.IsError || stats.IsCompact {
		t.Errorf("unexpected flags: %+v", stats)
	}
}

// Non-file items carry no stats, and must not inherit them from a prior item.
func TestRenderPreparedNonFileItemHasZeroStats(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("a.txt", []byte("one\ntwo\n"))
	opts := core.RenderOptions{Head: -1}
	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, PreparedItem{
		Raw: FileItem{File: file, Opts: opts}, Section: &core.SectionContext{}, FileIndexGlobal: 1,
	}, nil); err != nil {
		t.Fatal(err)
	}

	stats, err := proc.RenderPrepared(&buf, PreparedItem{
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

	_, _, from, lang, _ := applyConverters(slog.Default(), file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
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

	if _, _, from, _, _ := applyConverters(slog.Default(), file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), true); from != "" {
		t.Errorf("convertedFrom = %q, want empty while embedding", from)
	}
	if _, _, from, _, _ := applyConverters(slog.Default(), file, opts, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false); from != "png" {
		t.Errorf("convertedFrom = %q, want png when not embedding", from)
	}
}

func TestApplyConvertersReportsNoLanguageWhenNoneApply(t *testing.T) {
	src := []byte("package main\n")
	file := sources.NewBufferInputFile("main.go", src)

	_, _, from, lang, _ := applyConverters(slog.Default(), file, core.RenderOptions{}, internal.Classify(file.Path, src), bytes.NewReader(src), int64(len(src)), false)
	if from != "" || lang != "" {
		t.Errorf("got from=%q lang=%q, want both empty", from, lang)
	}
}

// An input with no extension is still converted when its content is media, and
// the conversion is what decides that it is no longer rendered as a picture.
func TestPrepareFileContextConvertsExtensionlessImage(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", "lx", "testdata", "media", "sample.png"))
	if err != nil {
		t.Fatal(err)
	}
	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("avatar", src)
	opts := core.RenderOptions{Head: -1, ExtractMedia: true}
	ctx, err := proc.prepareFileContext(file, opts, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.IsImage || ctx.IsBinary || ctx.ConvertedFrom != "png" {
		t.Errorf("IsImage=%v IsBinary=%v ConvertedFrom=%q, want a png converted to text",
			ctx.IsImage, ctx.IsBinary, ctx.ConvertedFrom)
	}
	if s, _ := ctx.Content.(string); !strings.Contains(s, "Container: png") {
		t.Errorf("content = %q", s)
	}
}

// Only bytes that bear the format out are rendered as a picture.
func TestClassifyVerifiesBeforeTreatingAsImage(t *testing.T) {
	scratch := make([]byte, 1024)
	cases := []struct {
		path string
		data []byte
		want bool
	}{
		{"logo.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), true},
		{"corrupt.png", []byte{0, 1, 2, 3, 4}, false},
		{"counts.dat", append([]byte("BM"), make([]byte, 60)...), false},
	}
	for _, c := range cases {
		format, verified := classify(c.path, bytes.NewReader(c.data), scratch)
		if got := format.IsImage() && verified; got != c.want {
			t.Errorf("%s: image=%v, want %v", c.path, got, c.want)
		}
	}
}

// bigLines is a text file well past the 32 KB that row counts were once
// estimated from, with every line distinct so its position can be checked.
func bigLines(n int) []byte {
	var b bytes.Buffer
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %05d %s\n", i, strings.Repeat("-", 20))
	}
	return b.Bytes()
}

func TestPrepareFileContextCountsBigFileExactly(t *testing.T) {
	data := bigLines(3000)
	if len(data) <= 32*1024 {
		t.Fatalf("fixture is %d bytes, want more than 32 KB", len(data))
	}
	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	for _, cfg := range []core.RenderOptions{
		{Head: -1},         // whole file
		{Head: 5},          // head only
		{Head: 5, Tail: 5}, // sliced
		{Tail: 5},          // tail only
		{Head: 0, Tail: 0}, // compact
	} {
		file := sources.NewBufferInputFile("big.txt", data)
		ctx, err := proc.prepareFileContext(file, cfg, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if ctx.TotalRows != 3000 || ctx.IsEstimate {
			t.Errorf("head=%d tail=%d: rows=%d estimate=%v, want exactly 3000",
				cfg.Head, cfg.Tail, ctx.TotalRows, ctx.IsEstimate)
		}
	}
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
	proc := NewProcessor(engine, core.GlobalContext{}, nil)
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
		ctx, err := proc.prepareFileContext(file, opts, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := ctx.Content.(string); got != want {
			t.Errorf("%s: content =\n%s\nwant\n%s", name, got, want)
		}
		if ctx.TotalRows != 3000 || ctx.IsEstimate {
			t.Errorf("%s: rows=%d estimate=%v, want exactly 3000", name, ctx.TotalRows, ctx.IsEstimate)
		}
	}
}

// A head and tail that overlap render the file once, with its line endings.
func TestPrepareFileContextOverlapRendersWholeFile(t *testing.T) {
	data := []byte("a\r\nb\r\nc\r\n")
	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("crlf.txt", data)
	opts := core.RenderOptions{Head: 2, Tail: 2}
	ctx, err := proc.prepareFileContext(file, opts, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := ctx.Content.(string); got != string(data) {
		t.Errorf("content = %q, want the whole file %q with CRLF kept", got, data)
	}
	if ctx.TotalRows != 3 || ctx.IsEstimate {
		t.Errorf("rows=%d estimate=%v, want exactly 3", ctx.TotalRows, ctx.IsEstimate)
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
		Open: func() (io.ReadCloser, error) { return h, nil },
	}
	opts := core.RenderOptions{Head: 2, Tail: 2, LineNumbers: true}

	engine, _ := templatex.Compile(core.NewConfig())
	proc := NewProcessor(engine, core.GlobalContext{}, nil)
	ctx, err := proc.prepareFileContext(file, opts, 1, nil)
	if err != nil {
		t.Fatal(err)
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
