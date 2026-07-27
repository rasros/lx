package render

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

func TestProcessor_RenderFile_Slicing(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	global := core.GlobalContext{TotalFiles: 1}

	proc := NewProcessor(engine, global, nil)

	file := sources.NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))
	file.Config = core.RunnerConfig{Head: 1, Tail: 1}

	scratch := make([]byte, 1024)
	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

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

func TestProcessor_RenderFile_SkeletonSlicingUsesFilteredRows(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	global := core.GlobalContext{TotalFiles: 1}

	proc := NewProcessor(engine, global, nil)

	file := sources.NewBufferInputFile("skeleton.go", []byte(`package p

func A() {
	println(1)
}

func B() {
	println(2)
}

func C() {
	println(3)
}
`))
	file.Config = core.RunnerConfig{Head: 1, Tail: 1, SkeletonFunctions: true}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "(13 rows, function signatures)") {
		t.Fatalf("expected original row count in header, got:\n%s", got)
	}
	if !strings.Contains(got, "... (1 rows skipped)") {
		t.Fatalf("expected gap based on filtered rows, got:\n%s", got)
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

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

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
	item := PreparedItem{Raw: file, Section: &core.SectionContext{}}

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
	file.Config = core.RunnerConfig{Head: -1, Tail: -1}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

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
	file.Config = core.RunnerConfig{Head: 0, Tail: 0}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

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
	file.Config = core.RunnerConfig{Head: -1, Tail: -1, LineNumbers: true}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

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

	reader, size, from := applyConverters(file, bytes.NewReader(src), int64(len(src)))
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
		name:    "test",
		applies: func(f sources.InputFile) bool { return strings.HasSuffix(f.Path, ".zzz") },
		convert: func(path string, r io.ReaderAt, size int64) ([]byte, error) {
			return []byte("converted"), nil
		},
	})
	t.Cleanup(func() { converters = converters[:len(converters)-1] })

	file := sources.NewBufferInputFile("doc.zzz", []byte("raw"))
	reader, size, from := applyConverters(file, bytes.NewReader([]byte("raw")), 3)

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
		name:    "failing",
		applies: func(f sources.InputFile) bool { return strings.HasSuffix(f.Path, ".zzz") },
		convert: func(path string, r io.ReaderAt, size int64) ([]byte, error) {
			return nil, io.ErrUnexpectedEOF
		},
	})
	t.Cleanup(func() { converters = converters[:len(converters)-1] })

	src := []byte("raw content")
	reader, size, from := applyConverters(
		sources.NewBufferInputFile("doc.zzz", src), bytes.NewReader(src), int64(len(src)))

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
	cfg := core.RunnerConfig{Head: -1, SkeletonFunctions: true}

	reader, size, mode, originalRows := applySkeleton(
		cfg, "go", bytes.NewReader(src), int64(len(src)), false, false)

	if mode != "function signatures" {
		t.Errorf("mode = %q", mode)
	}
	if originalRows != 5 {
		t.Errorf("originalRows = %d, want 5", originalRows)
	}
	if size >= int64(len(src)) {
		t.Errorf("filtered size %d should be smaller than %d", size, len(src))
	}
	if reader == nil {
		t.Error("reader is nil")
	}
}

func TestApplySkeletonSkipsBinaryAndImages(t *testing.T) {
	src := []byte("package main\nfunc A() {}\n")
	cfg := core.RunnerConfig{Head: -1, SkeletonFunctions: true}

	for _, tc := range []struct {
		name              string
		isBinary, isImage bool
	}{
		{"binary", true, false},
		{"image", false, true},
	} {
		_, size, mode, originalRows := applySkeleton(
			cfg, "go", bytes.NewReader(src), int64(len(src)), tc.isBinary, tc.isImage)
		if mode != "" || originalRows != -1 || size != int64(len(src)) {
			t.Errorf("%s: skeleton ran anyway (mode=%q rows=%d size=%d)", tc.name, mode, originalRows, size)
		}
	}
}

func TestRenderPreparedReportsStats(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil)

	file := sources.NewBufferInputFile("a.txt", []byte("one\ntwo\nthree\n"))
	file.Config = core.RunnerConfig{Head: -1}

	var buf bytes.Buffer
	stats, err := proc.RenderPrepared(&buf, PreparedItem{
		Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1,
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
	file.Config = core.RunnerConfig{Head: -1}
	var buf bytes.Buffer
	if _, err := proc.RenderPrepared(&buf, PreparedItem{
		Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1,
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
