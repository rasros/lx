package streaming

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

const docxRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`

func makeTestDOCX(t *testing.T, paragraphs ...string) []byte {
	t.Helper()
	var xmlBuf strings.Builder
	xmlBuf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	xmlBuf.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	xmlBuf.WriteString(`<w:body>`)
	for _, p := range paragraphs {
		fmt.Fprintf(&xmlBuf, `<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, p)
	}
	xmlBuf.WriteString(`</w:body></w:document>`)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeEntry := func(name, content string) {
		t.Helper()
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	writeEntry("word/document.xml", xmlBuf.String())
	writeEntry("word/_rels/document.xml.rels", docxRelsXML)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStream_ExtractDocuments_PropagatesFromRunnerConfig(t *testing.T) {
	cfg := core.NewConfig()
	stream, err := NewStream(cfg, core.RunnerConfig{Head: -1, ExtractDocuments: false})
	if err != nil {
		t.Fatal(err)
	}

	data := makeTestDOCX(t, "Content")
	stream.AddFile(sources.NewBufferInputFile("doc.docx", data))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if strings.Contains(got, "Content") {
		t.Error("extraction should be disabled when config.ExtractDocuments=false")
	}
}

func TestStream_ExtractDocuments_EnabledViaRunnerConfig(t *testing.T) {
	cfg := core.NewConfig()
	stream, err := NewStream(cfg, core.RunnerConfig{Head: -1, ExtractDocuments: true})
	if err != nil {
		t.Fatal(err)
	}

	data := makeTestDOCX(t, "DefaultEnabled")
	stream.AddFile(sources.NewBufferInputFile("doc.docx", data))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
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
	stream, err := NewStream(cfg, core.RunnerConfig{Head: -1, ExtractDocuments: true})
	if err != nil {
		t.Fatal(err)
	}

	stream.AddFile(sources.NewBufferInputFile("doc.docx", makeTestDOCX(t, "Content")))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[docx]" {
		t.Errorf("ConvertedFrom rendered as %q, want %q", got, "[docx]")
	}
}

func TestStream_ConvertedFromEmptyWithoutAConverter(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "[{{ .ConvertedFrom }}]"
	stream, err := NewStream(cfg, core.RunnerConfig{Head: -1})
	if err != nil {
		t.Fatal(err)
	}

	stream.AddFile(sources.NewBufferInputFile("main.go", []byte("package main\n")))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("ConvertedFrom rendered as %q, want empty", got)
	}
}
