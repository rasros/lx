package sources

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func makeTestPPTX(t *testing.T, slides ...string) []byte {
	t.Helper()
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

	writeEntry("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`</Types>`)

	for i, text := range slides {
		name := fmt.Sprintf("ppt/slides/slide%d.xml", i+1)
		content := fmt.Sprintf(
			`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
				`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`+
				` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`+
				`<p:cSld><p:spTree><p:sp><p:txBody>`+
				`<a:p><a:r><a:t>%s</a:t></a:r></a:p>`+
				`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`, text)
		writeEntry(name, content)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

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

func makeTestPDF(t *testing.T, text string) []byte {
	t.Helper()
	stream := "BT /F1 12 Tf 100 700 Td (" + text + ") Tj ET\n"
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n" +
			"/Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", len(stream), stream),
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, obj := range objs {
		offsets[i] = buf.Len()
		buf.WriteString(obj)
	}
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objs)+1)
	fmt.Fprintf(&buf, "0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", len(objs)+1)
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefOffset)
	return buf.Bytes()
}

func TestIsDocumentPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"report.pdf", true},
		{"letter.docx", true},
		{"budget.xlsx", true},
		{"slides.pptx", true},
		{"REPORT.PDF", true},
		{"Letter.DOCX", true},
		{"Budget.XLSX", true},
		{"docs/report.pdf", true},
		{"dir/sub/letter.docx", true},
		{"text.odt", false},
		{"sheet.ods", false},
		{"pres.odp", false},
		{"legacy.doc", false},
		{"old.xls", false},
		{"main.go", false},
		{"readme.txt", false},
		{"noextension", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := IsDocumentPath(tc.path)
			if got != tc.want {
				t.Errorf("IsDocumentPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestDocxXMLToText_SingleParagraph(t *testing.T) {
	xml := `<w:body><w:p><w:r><w:t>Hello World</w:t></w:r></w:p></w:body>`
	got := xmlToText(strings.NewReader(xml))
	if !strings.Contains(got, "Hello World") {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestDocxXMLToText_ParagraphEndsWithNewline(t *testing.T) {
	xml := `<w:body><w:p><w:r><w:t>Line 1</w:t></w:r></w:p><w:p><w:r><w:t>Line 2</w:t></w:r></w:p></w:body>`
	got := xmlToText(strings.NewReader(xml))
	if !strings.Contains(got, "\n") {
		t.Errorf("expected newline between paragraphs, got %q", got)
	}
	if !strings.Contains(got, "Line 1") {
		t.Errorf("expected 'Line 1', got %q", got)
	}
	if !strings.Contains(got, "Line 2") {
		t.Errorf("expected 'Line 2', got %q", got)
	}
}

func TestDocxXMLToText_MultipleRuns(t *testing.T) {
	xml := `<w:p><w:r><w:t>Hello </w:t></w:r><w:r><w:t>World</w:t></w:r></w:p>`
	got := xmlToText(strings.NewReader(xml))
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("expected both run texts, got %q", got)
	}
}

func TestDocxXMLToText_Empty(t *testing.T) {
	got := xmlToText(strings.NewReader(""))
	if got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestDocxXMLToText_InvalidXML_DoesNotPanic(t *testing.T) {
	got := xmlToText(strings.NewReader("<unclosed <tag >>"))
	_ = got
}

func TestDocxXMLToText_NoText(t *testing.T) {
	xml := `<w:body><w:p><w:pPr><w:jc w:val="center"/></w:pPr></w:p></w:body>`
	got := xmlToText(strings.NewReader(xml))
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected no text content, got %q", got)
	}
}

func TestExtractDOCXText_SingleParagraph(t *testing.T) {
	data := makeTestDOCX(t, "Hello DOCX")
	r := bytes.NewReader(data)
	text, err := extractDOCXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "Hello DOCX") {
		t.Errorf("expected 'Hello DOCX' in extracted text, got %q", string(text))
	}
}

func TestExtractDOCXText_MultipleParagraphs(t *testing.T) {
	data := makeTestDOCX(t, "First paragraph", "Second paragraph", "Third paragraph")
	r := bytes.NewReader(data)
	text, err := extractDOCXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(text)
	for _, want := range []string{"First paragraph", "Second paragraph", "Third paragraph"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in extracted text, got:\n%s", want, got)
		}
	}
}

func TestExtractDOCXText_Corrupted(t *testing.T) {
	data := []byte("this is not a zip or docx file")
	r := bytes.NewReader(data)
	_, err := extractDOCXText(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted DOCX, got nil")
	}
}

func TestExtractDOCXText_EmptyZip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	w.Create("other.txt")
	w.Close()
	data := buf.Bytes()
	r := bytes.NewReader(data)
	text, _ := extractDOCXText(r, int64(len(data)))
	_ = text
}

func TestExtractXLSXText_CellValues(t *testing.T) {
	data := makeTestXLSX(t, "Sheet1", map[string]interface{}{
		"A1": "Name",
		"B1": "Score",
		"A2": "Alice",
		"B2": 42,
	})
	r := bytes.NewReader(data)
	text, err := extractXLSXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(text)
	for _, want := range []string{"Sheet1", "Name", "Score", "Alice", "42"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in extracted text, got:\n%s", want, got)
		}
	}
}

func TestExtractXLSXText_SheetNameInOutput(t *testing.T) {
	data := makeTestXLSX(t, "SalesData", map[string]interface{}{"A1": "Q1"})
	r := bytes.NewReader(data)
	text, err := extractXLSXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "SalesData") {
		t.Errorf("expected sheet name 'SalesData' in output, got:\n%s", string(text))
	}
}

func TestExtractXLSXText_MultipleSheets(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", "sheet-one")
	f.NewSheet("Sheet2")
	f.SetCellValue("Sheet2", "A1", "sheet-two")
	var buf bytes.Buffer
	f.Write(&buf)
	data := buf.Bytes()

	r := bytes.NewReader(data)
	text, err := extractXLSXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(text)
	if !strings.Contains(got, "sheet-one") {
		t.Errorf("expected 'sheet-one', got:\n%s", got)
	}
	if !strings.Contains(got, "sheet-two") {
		t.Errorf("expected 'sheet-two', got:\n%s", got)
	}
}

func TestExtractXLSXText_RowsSeparatedByTabs(t *testing.T) {
	data := makeTestXLSX(t, "Sheet1", map[string]interface{}{
		"A1": "Col1",
		"B1": "Col2",
		"C1": "Col3",
	})
	r := bytes.NewReader(data)
	text, err := extractXLSXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "\t") {
		t.Errorf("expected tab-separated columns, got:\n%s", string(text))
	}
}

func TestExtractXLSXText_Corrupted(t *testing.T) {
	data := []byte("not a xlsx file at all")
	r := bytes.NewReader(data)
	_, err := extractXLSXText(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted XLSX, got nil")
	}
}

func TestExtractPDFText_ValidPDF_NoError(t *testing.T) {
	data := makeTestPDF(t, "TestContent")
	r := bytes.NewReader(data)
	_, err := extractPDFText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error from valid PDF: %v", err)
	}
}

func TestExtractPDFText_Corrupted(t *testing.T) {
	data := []byte("this is definitely not a pdf")
	r := bytes.NewReader(data)
	_, err := extractPDFText(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted PDF, got nil")
	}
}

func TestExtractPDFText_TruncatedHeader(t *testing.T) {
	data := []byte("%PDF-1.4")
	r := bytes.NewReader(data)
	_, err := extractPDFText(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for truncated PDF, got nil")
	}
}

func TestExtractDocumentText_UnsupportedExtension(t *testing.T) {
	data := []byte("some data")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText("file.odt", r, int64(len(data)))
	if err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error message, got: %v", err)
	}
}

func TestExtractDocumentText_DispatchToPDF(t *testing.T) {
	data := []byte("garbage")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText("file.pdf", r, int64(len(data)))
	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Errorf("pdf extension was not dispatched: %v", err)
	}
}

func TestExtractDocumentText_DispatchToDOCX(t *testing.T) {
	data := []byte("garbage")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText("file.docx", r, int64(len(data)))
	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Errorf("docx extension was not dispatched: %v", err)
	}
}

func TestExtractDocumentText_DispatchToXLSX(t *testing.T) {
	data := []byte("garbage")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText("file.xlsx", r, int64(len(data)))
	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Errorf("xlsx extension was not dispatched: %v", err)
	}
}

func TestExtractDocumentText_DispatchToPPTX(t *testing.T) {
	data := []byte("garbage")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText("file.pptx", r, int64(len(data)))
	if err != nil && strings.Contains(err.Error(), "unsupported") {
		t.Errorf("pptx extension was not dispatched: %v", err)
	}
}

func TestExtractPPTXText_SingleSlide(t *testing.T) {
	data := makeTestPPTX(t, "Hello PPTX World")
	r := bytes.NewReader(data)
	text, err := extractPPTXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "Hello PPTX World") {
		t.Errorf("expected 'Hello PPTX World', got %q", string(text))
	}
}

func TestExtractPPTXText_MultipleSlides(t *testing.T) {
	data := makeTestPPTX(t, "Slide one content", "Slide two content", "Slide three content")
	r := bytes.NewReader(data)
	text, err := extractPPTXText(r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(text)
	for _, want := range []string{"Slide one content", "Slide two content", "Slide three content"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in extracted text, got:\n%s", want, got)
		}
	}
}

func TestExtractPPTXText_Corrupted(t *testing.T) {
	data := []byte("not a pptx file")
	r := bytes.NewReader(data)
	_, err := extractPPTXText(r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted PPTX, got nil")
	}
}

func TestExtractDocumentText_CaseInsensitiveExtension(t *testing.T) {
	data := makeTestDOCX(t, "case test")
	r := bytes.NewReader(data)
	text, err := ExtractDocumentText("LETTER.DOCX", r, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "case test") {
		t.Errorf("expected 'case test', got %q", string(text))
	}
}
