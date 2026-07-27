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

	proc := NewProcessor(engine, global, nil, "markdown")

	file := sources.NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))
	file.Config = core.RunnerConfig{Head: 1, Tail: 1}

	scratch := make([]byte, 1024)
	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, scratch); err != nil {
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

	proc := NewProcessor(engine, global, nil, "markdown")

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
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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

	proc := NewProcessor(engine, core.GlobalContext{}, nil, "html")

	file, err := sources.NewInputFileFromPath(os.DirFS(tmp), imgName)
	if err != nil {
		t.Fatal(err)
	}
	file.AbsPath = imgPath

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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

	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	file := sources.InputFile{Path: "ghost.txt", Open: func() (io.ReadCloser, error) { return nil, os.ErrPermission }}
	item := PreparedItem{Raw: file, Section: &core.SectionContext{}}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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
	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	binaryContent := append([]byte("ELF"), 0x00, 0x01, 0x02, 0x03)
	file := sources.NewBufferInputFile("program", binaryContent)
	file.Config = core.RunnerConfig{Head: -1, Tail: -1}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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
	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	file := sources.NewBufferInputFile("data.txt", []byte("lots of content\n"))
	file.Config = core.RunnerConfig{Head: 0, Tail: 0}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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
	proc := NewProcessor(engine, core.GlobalContext{TotalFiles: 1}, nil, "markdown")

	file := sources.NewBufferInputFile("code.go", []byte("package main\nfunc main() {}\n"))
	file.Config = core.RunnerConfig{Head: -1, Tail: -1, LineNumbers: true}

	item := PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
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
