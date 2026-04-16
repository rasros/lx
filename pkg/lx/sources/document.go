package sources

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

var documentSuffixes = []string{".pdf", ".docx", ".xlsx", ".pptx"}

// IsDocumentPath reports whether the path has a document file extension.
func IsDocumentPath(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range documentSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// ExtractDocumentText extracts plain text from a document file.
func ExtractDocumentText(path string, r io.ReaderAt, size int64) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractPDFText(r, size)
	case ".docx":
		return extractDOCXText(r, size)
	case ".xlsx":
		return extractXLSXText(r, size)
	case ".pptx":
		return extractPPTXText(r, size)
	}
	return nil, fmt.Errorf("unsupported document type: %s", ext)
}

func ExtractPDFText(r io.ReaderAt, size int64) ([]byte, error)  { return extractPDFText(r, size) }
func ExtractDOCXText(r io.ReaderAt, size int64) ([]byte, error) { return extractDOCXText(r, size) }
func ExtractXLSXText(r io.ReaderAt, size int64) ([]byte, error) { return extractXLSXText(r, size) }
func ExtractPPTXText(r io.ReaderAt, size int64) ([]byte, error) { return extractPPTXText(r, size) }
func XMLToText(r io.Reader) string                              { return xmlToText(r) }

func extractPDFText(r io.ReaderAt, size int64) ([]byte, error) {
	reader, err := pdflib.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	textReader, err := reader.GetPlainText()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(textReader)
}

func extractDOCXText(r io.ReaderAt, size int64) ([]byte, error) {
	doc, err := docx.ReadDocxFromMemory(r, size)
	if err != nil {
		return nil, err
	}
	defer doc.Close()
	return []byte(xmlToText(strings.NewReader(doc.Editable().GetContent()))), nil
}

func extractPPTXText(r io.ReaderAt, size int64) ([]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	// Collect slide XML files from ppt/slides/slide<N>.xml.
	type numberedFile struct {
		n int
		f *zip.File
	}
	var slides []numberedFile
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		// Extract the numeric part between "slide" and ".xml".
		base := strings.TrimPrefix(name, "ppt/slides/slide")
		base = strings.TrimSuffix(base, ".xml")
		n, err := strconv.Atoi(base)
		if err != nil {
			continue
		}
		slides = append(slides, numberedFile{n, f})
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].n < slides[j].n })

	var sb strings.Builder
	for _, s := range slides {
		rc, err := s.f.Open()
		if err != nil {
			continue
		}
		sb.WriteString(xmlToText(rc))
		rc.Close()
	}
	return []byte(sb.String()), nil
}

// xmlToText extracts plain text from XML content.
// Paragraph ends (</p> in any namespace) become newlines.
func xmlToText(r io.Reader) string {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "p" {
				sb.WriteByte('\n')
			}
		case xml.CharData:
			sb.Write(t)
		}
	}
	return sb.String()
}

func extractXLSXText(r io.ReaderAt, size int64) ([]byte, error) {
	f, err := excelize.OpenReader(io.NewSectionReader(r, 0, size))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "%s:\n", sheet)
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteByte('\n')
		}
	}
	return []byte(sb.String()), nil
}
