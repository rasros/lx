package sources

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rasros/lx/internal/testzip"
	"github.com/xuri/excelize/v2"
)

func makeTestPPTX(t *testing.T, slides ...string) []byte {
	t.Helper()
	entries := [][2]string{{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>`}}
	for i, text := range slides {
		entries = append(entries, [2]string{fmt.Sprintf("ppt/slides/slide%d.xml", i+1), fmt.Sprintf(
			`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
				`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`+
				` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`+
				`<p:cSld><p:spTree><p:sp><p:txBody>`+
				`<a:p><a:r><a:t>%s</a:t></a:r></a:p>`+
				`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`, text)})
	}
	return testzip.Bytes(t, entries...)
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
		{"slides.pptx", true},
		{"Letter.DOCX", true},
		{"docs/page.html", true},
		{"text.odt", false},
		{"legacy.doc", false},
		{"main.go", false},
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

func TestIsDocumentInput_PDFMediaTypeWithoutExtension(t *testing.T) {
	mediaType := "application/pdf"
	f := InputFile{Path: "https://arxiv.org/pdf/2608.26263", mediaType: &mediaType}
	if !IsDocumentInput(f) {
		t.Errorf("IsDocumentInput(%q with media type %q) = false, want true", f.Path, mediaType)
	}
}

func TestIsDocumentInput_UnrelatedMediaTypeWithoutExtension(t *testing.T) {
	mediaType := "application/octet-stream"
	f := InputFile{Path: "https://example.com/download", mediaType: &mediaType}
	if IsDocumentInput(f) {
		t.Errorf("IsDocumentInput(%q with media type %q) = true, want false", f.Path, mediaType)
	}
}

func TestIsDocumentInput_PathExtensionWithNoMediaType(t *testing.T) {
	f := InputFile{Path: "report.pdf"}
	if !IsDocumentInput(f) {
		t.Error("IsDocumentInput(report.pdf) = false, want true")
	}
}

func TestConvertInput_PDFMediaTypeWithoutExtension(t *testing.T) {
	data := makeTestPDF(t, "URLPdfContent")
	mediaType := "application/pdf; charset=binary"
	f := InputFile{Path: "https://arxiv.org/pdf/2608.26263", mediaType: &mediaType}
	text, err := ConvertInput(context.Background(), f, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(text), "URLPdfContent") {
		t.Errorf("expected extracted PDF text, got: %q", string(text))
	}
}

func TestDocxXMLToText_ParagraphEndsWithNewline(t *testing.T) {
	xml := `<w:body><w:p><w:r><w:t>Line 1</w:t></w:r></w:p><w:p><w:r><w:t>Line 2</w:t></w:r></w:p></w:body>`
	got := xmlToText(context.Background(), strings.NewReader(xml))
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
	got := xmlToText(context.Background(), strings.NewReader(xml))
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("expected both run texts, got %q", got)
	}
}

func TestDocxXMLToText_Empty(t *testing.T) {
	got := xmlToText(context.Background(), strings.NewReader(""))
	if got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestExtractDOCXText_MultipleParagraphs(t *testing.T) {
	data := testzip.DOCX(t, "First paragraph", "Second paragraph", "Third paragraph")
	r := bytes.NewReader(data)
	text, err := extractDOCXText(context.Background(), r, int64(len(data)))
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
	_, err := extractDOCXText(context.Background(), r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted DOCX, got nil")
	}
}

func TestExtractXLSXText_Corrupted(t *testing.T) {
	data := []byte("not a xlsx file at all")
	r := bytes.NewReader(data)
	_, err := extractXLSXText(context.Background(), r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted XLSX, got nil")
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

func TestExtractDocumentText_UnsupportedExtension(t *testing.T) {
	data := []byte("some data")
	r := bytes.NewReader(data)
	_, err := ExtractDocumentText(context.Background(), "file.odt", r, int64(len(data)))
	if err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error message, got: %v", err)
	}
}

func TestExtractPPTXText_MultipleSlides(t *testing.T) {
	data := makeTestPPTX(t, "Slide one content", "Slide two content", "Slide three content")
	r := bytes.NewReader(data)
	text, err := extractPPTXText(context.Background(), r, int64(len(data)))
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
	_, err := extractPPTXText(context.Background(), r, int64(len(data)))
	if err == nil {
		t.Error("expected error for corrupted PPTX, got nil")
	}
}

func TestIsHTMLPath(t *testing.T) {
	for _, p := range []string{"a.html", "a.HTM", "dir/b.xhtml"} {
		if !isHTMLPath(p) {
			t.Errorf("isHTMLPath(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"a.pdf", "a.htmlx", "a.txt", "html"} {
		if isHTMLPath(p) {
			t.Errorf("isHTMLPath(%q) = true, want false", p)
		}
	}
}

// cancellingReader cancels the run on its first read, as a signal arriving
// mid-conversion would.
type cancellingReader struct {
	r      *bytes.Reader
	cancel context.CancelFunc
}

func (c cancellingReader) ReadAt(p []byte, off int64) (int, error) {
	c.cancel()
	return c.r.ReadAt(p, off)
}

// A conversion cancelled midway returns ctx's error, never what the parse
// made of its cut-short input.
func TestExtractDocumentTextStopsWhenCancelled(t *testing.T) {
	docs := map[string][]byte{
		"a.pdf":  makeTestPDF(t, "pdf text"),
		"a.docx": testzip.DOCX(t, "docx text"),
		"a.xlsx": makeTestXLSX(t, "Sheet1", map[string]interface{}{"A1": "xlsx text"}),
		"a.pptx": makeTestPPTX(t, "pptx text"),
		"a.html": []byte("<p>html text</p>"),
	}
	for name, data := range docs {
		ctx, cancel := context.WithCancel(context.Background())
		r := cancellingReader{r: bytes.NewReader(data), cancel: cancel}
		text, err := ExtractDocumentText(ctx, name, r, int64(len(data)))
		if !errors.Is(err, context.Canceled) || text != nil {
			t.Errorf("%s: got %q, %v; want context.Canceled", name, text, err)
		}
	}
}

// Streaming a sheet's rows renders them as GetRows returns them: empty rows
// between rows with cells kept, empty rows at the end dropped.
func TestXLSXSheetTextMatchesGetRows(t *testing.T) {
	data := makeTestXLSX(t, "Sheet1", map[string]interface{}{
		"B2": "b2", "A5": "a5", "C5": 3, "A7": "",
	})
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, row := range rows {
		want.WriteString(strings.Join(row, "\t") + "\n")
	}
	got, err := xlsxSheetText(context.Background(), f, "Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if got != want.String() {
		t.Errorf("xlsxSheetText = %q, want %q", got, want.String())
	}
}

// A sheet stops between rows once ctx is done, not only between sheets.
func TestXLSXSheetTextStopsWithinSheet(t *testing.T) {
	cells := map[string]interface{}{}
	for i := 1; i <= 3*ctxCheckEvery; i++ {
		cells[fmt.Sprintf("A%d", i)] = i
	}
	f, err := excelize.OpenReader(bytes.NewReader(makeTestXLSX(t, "Sheet1", cells)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, _ := xlsxSheetText(ctx, f, "Sheet1")
	if n := strings.Count(got, "\n"); n >= ctxCheckEvery {
		t.Errorf("read %d rows of a cancelled sheet, want fewer than %d", n, ctxCheckEvery)
	}
}
