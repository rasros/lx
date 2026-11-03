package streaming

import (
	"context"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/testzip"
	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

func TestStream_ExtractDocuments_PropagatesFromRenderOptions(t *testing.T) {
	cfg := core.NewConfig()
	opts := core.RenderOptions{Head: -1, ExtractDocuments: false}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatal(err)
	}

	data := testzip.DOCX(t, "Content")
	stream.AddFile(sources.NewBufferInputFile("doc.docx", data), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if strings.Contains(got, "Content") {
		t.Error("extraction should be disabled when config.ExtractDocuments=false")
	}
}

func TestStream_ExtractDocuments_EnabledViaRenderOptions(t *testing.T) {
	cfg := core.NewConfig()
	opts := core.RenderOptions{Head: -1, ExtractDocuments: true}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatal(err)
	}

	data := testzip.DOCX(t, "DefaultEnabled")
	stream.AddFile(sources.NewBufferInputFile("doc.docx", data), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "DefaultEnabled") {
		t.Errorf("extraction should be enabled by default, got:\n%s", got)
	}
}

func TestStream_ConvertedFromNamesTheSourceFormat(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "[{{ .ConvertedFrom }}]"
	opts := core.RenderOptions{Head: -1, ExtractDocuments: true}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatal(err)
	}

	stream.AddFile(sources.NewBufferInputFile("doc.docx", testzip.DOCX(t, "Content")), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[docx]" {
		t.Errorf("ConvertedFrom rendered as %q, want %q", got, "[docx]")
	}
}

func TestStream_ConvertedFromEmptyWithoutAConverter(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "[{{ .ConvertedFrom }}]"
	opts := core.RenderOptions{Head: -1}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatal(err)
	}

	stream.AddFile(sources.NewBufferInputFile("main.go", []byte("package main\n")), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("ConvertedFrom rendered as %q, want empty", got)
	}
}
