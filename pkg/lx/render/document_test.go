package render

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
	"github.com/xuri/excelize/v2"
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

func makeTestXLSX(t *testing.T, sheet string, cells map[string]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if sheet != "Sheet1" {
		f.NewSheet(sheet)
		f.DeleteSheet("Sheet1")
	}
	for cell, val := range cells {
		if err := f.SetCellValue(sheet, cell, val); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcessor_ExtractDocuments_DOCX(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	data := makeTestDOCX(t, "Important document content")
	file := sources.NewBufferInputFile("report.docx", data)
	file.Config = core.RunnerConfig{Head: -1, ExtractDocuments: true}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	if strings.Contains(got, "binary file skipped") {
		t.Error("DOCX should be rendered as text, not binary")
	}
	if !strings.Contains(got, "Important document content") {
		t.Errorf("expected extracted text in output, got:\n%s", got)
	}
}

func TestProcessor_ExtractDocuments_XLSX(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	data := makeTestXLSX(t, "Sheet1", map[string]interface{}{"A1": "Revenue", "B1": "12345"})
	file := sources.NewBufferInputFile("budget.xlsx", data)
	file.Config = core.RunnerConfig{Head: -1, ExtractDocuments: true}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	if strings.Contains(got, "binary file skipped") {
		t.Error("XLSX should be rendered as text, not binary")
	}
	if !strings.Contains(got, "Revenue") {
		t.Errorf("expected extracted cell value in output, got:\n%s", got)
	}
}

func TestProcessor_NoExtractDocuments_DOCX(t *testing.T) {
	cfg := core.NewConfig()
	engine, _ := templatex.Compile(cfg)
	proc := NewProcessor(engine, core.GlobalContext{}, nil, "markdown")

	data := makeTestDOCX(t, "Should not appear")
	file := sources.NewBufferInputFile("report.docx", data)
	file.Config = core.RunnerConfig{Head: -1, ExtractDocuments: false}

	var buf bytes.Buffer
	if err := proc.RenderPrepared(&buf, PreparedItem{Raw: file, Section: &core.SectionContext{}, FileIndexGlobal: 1}, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	if !strings.Contains(got, "binary file skipped") {
		t.Errorf("DOCX should be treated as binary when extraction is disabled, got:\n%s", got)
	}
}
