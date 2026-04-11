package lx

import (
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

var documentSuffixes = []string{".pdf", ".docx", ".xlsx"}

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
	}
	return nil, fmt.Errorf("unsupported document type: %s", ext)
}

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
	xmlContent := doc.Editable().GetContent()
	return []byte(docxXMLToText(xmlContent)), nil
}

// docxXMLToText extracts plain text from DOCX body XML content.
// Paragraph ends (<w:p/>) become newlines.
func docxXMLToText(xmlContent string) string {
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))
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
