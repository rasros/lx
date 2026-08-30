package sources

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/rasros/lx/pkg/lx/internal"
	"github.com/xuri/excelize/v2"
)

var documentSuffixes = []string{".pdf", ".docx", ".xlsx", ".pptx", ".html", ".htm", ".xhtml"}

// isHTMLPath reports whether the path is an HTML document.
func isHTMLPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") ||
		strings.HasSuffix(lower, ".xhtml")
}

// IsHTMLInput reports whether an input should be treated as HTML. A suffix
// settles it for files; URLs frequently have none, so the media type their
// source reported decides those. A local file with neither — no HTML suffix and
// no declared type — is left as-is, since nothing here inspects content.
func IsHTMLInput(f InputFile) bool {
	if isHTMLPath(f.Path) {
		return true
	}
	return f.mediaType != nil && isHTMLMediaType(*f.mediaType)
}

func isHTMLMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

// ConvertInput converts a document input to text, choosing the HTML path by
// suffix or media type and falling back to the suffix-driven extractors.
func ConvertInput(f InputFile, r io.ReaderAt, size int64) ([]byte, error) {
	if IsHTMLInput(f) {
		return internal.HTMLToMarkdown(io.NewSectionReader(r, 0, size))
	}
	path := f.Path
	if !IsDocumentPath(path) && f.mediaType != nil {
		if suffix := documentSuffixForMediaType(*f.mediaType); suffix != "" {
			path += suffix
		}
	}
	return ExtractDocumentText(path, r, size)
}

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

// documentSuffixForMediaType maps a Content-Type to the suffix ExtractDocumentText dispatches on.
func documentSuffixForMediaType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	switch mediaType {
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	}
	return ""
}

// IsDocumentInput reports whether f should be converted by the document converter.
func IsDocumentInput(f InputFile) bool {
	if IsDocumentPath(f.Path) || IsHTMLInput(f) {
		return true
	}
	return f.mediaType != nil && documentSuffixForMediaType(*f.mediaType) != ""
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
	case ".html", ".htm", ".xhtml":
		return internal.HTMLToMarkdown(io.NewSectionReader(r, 0, size))
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
