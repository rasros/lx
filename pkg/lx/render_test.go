package lx

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessor_RenderFile_Slicing(t *testing.T) {
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)
	global := GlobalContext{TotalFiles: 1}

	proc := newProcessor(engine, global, nil, "markdown", false)

	file := NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))
	file.Config = RunnerConfig{Head: 1, Tail: 1}

	scratch := make([]byte, 1024)
	item := preparedItem{
		raw:             file,
		section:         &SectionContext{},
		fileIndexGlobal: 1,
	}

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
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)
	global := GlobalContext{TotalFiles: 1}

	proc := newProcessor(engine, global, nil, "markdown", false)

	file := NewBufferInputFile("skeleton.go", []byte(`package p

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
	file.Config = RunnerConfig{Head: 1, Tail: 1, SkeletonFunctions: true}

	item := preparedItem{
		raw:             file,
		section:         &SectionContext{},
		fileIndexGlobal: 1,
	}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "(3 rows, function signatures)") {
		t.Fatalf("expected filtered row count in header, got:\n%s", got)
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

	cfg := NewConfig()
	cfg.OutputFormat = "html"
	engine, _ := CompileTemplates(cfg)

	proc := newProcessor(engine, GlobalContext{}, nil, "html", false)

	file, err := NewInputFileFromPath(os.DirFS(tmp), imgName)
	if err != nil {
		t.Fatal(err)
	}
	file.AbsPath = imgPath

	item := preparedItem{
		raw:             file,
		section:         &SectionContext{},
		fileIndexGlobal: 1,
	}

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
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)

	proc := newProcessor(engine, GlobalContext{}, nil, "markdown", false)

	file := InputFile{
		Path: "ghost.txt",
		Open: func() (io.ReadCloser, error) { return nil, os.ErrPermission },
	}

	item := preparedItem{raw: file, section: &SectionContext{}}

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
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)
	proc := newProcessor(engine, GlobalContext{}, nil, "markdown", false)

	// Null byte triggers binary detection.
	binaryContent := append([]byte("ELF"), 0x00, 0x01, 0x02, 0x03)
	file := NewBufferInputFile("program", binaryContent)
	file.Config = RunnerConfig{Head: -1, Tail: -1}

	item := preparedItem{raw: file, section: &SectionContext{}, fileIndexGlobal: 1}

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
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)
	proc := newProcessor(engine, GlobalContext{}, nil, "markdown", false)

	file := NewBufferInputFile("data.txt", []byte("lots of content\n"))
	file.Config = RunnerConfig{Head: 0, Tail: 0} // triggers compact

	item := preparedItem{raw: file, section: &SectionContext{}, fileIndexGlobal: 1}

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
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)
	proc := newProcessor(engine, GlobalContext{TotalFiles: 1}, nil, "markdown", false)

	file := NewBufferInputFile("code.go", []byte("package main\nfunc main() {}\n"))
	file.Config = RunnerConfig{Head: -1, Tail: -1, LineNumbers: true}

	item := preparedItem{raw: file, section: &SectionContext{}, fileIndexGlobal: 1}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, item, nil); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "1: package main") {
		t.Errorf("Expected line numbers in output, got:\n%s", got)
	}
}
