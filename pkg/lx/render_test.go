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

	proc := newProcessor(engine, global, nil, "markdown")

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

	proc := newProcessor(engine, GlobalContext{}, nil, "html")

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

	proc := newProcessor(engine, GlobalContext{}, nil, "markdown")

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
