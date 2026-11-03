package sources

import (
	"archive/zip"
	"context"
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
// suffix or media type and falling back to the suffix-driven extractors. It
// stops reading once ctx is done and returns ctx's error.
func ConvertInput(ctx context.Context, f InputFile, r io.ReaderAt, size int64) ([]byte, error) {
	if IsHTMLInput(f) {
		return htmlToMarkdown(ctx, r, size)
	}
	path := f.Path
	if !IsDocumentPath(path) && f.mediaType != nil {
		if suffix := documentSuffixForMediaType(*f.mediaType); suffix != "" {
			path += suffix
		}
	}
	return ExtractDocumentText(ctx, path, r, size)
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

// ExtractDocumentText extracts plain text from a document file. It stops
// reading once ctx is done and returns ctx's error.
func ExtractDocumentText(ctx context.Context, path string, r io.ReaderAt, size int64) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var (
		text []byte
		err  error
	)
	switch ext {
	case ".pdf":
		text, err = extractPDFText(internal.ContextReaderAt(ctx, r), size)
	case ".docx":
		text, err = extractDOCXText(ctx, internal.ContextReaderAt(ctx, r), size)
	case ".xlsx":
		text, err = extractXLSXText(ctx, internal.ContextReaderAt(ctx, r), size)
	case ".pptx":
		text, err = extractPPTXText(ctx, internal.ContextReaderAt(ctx, r), size)
	case ".html", ".htm", ".xhtml":
		return htmlToMarkdown(ctx, r, size)
	default:
		return nil, fmt.Errorf("unsupported document type: %s", ext)
	}
	return orCtxErr(ctx, text, err)
}

func htmlToMarkdown(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
	text, err := internal.HTMLToMarkdown(io.NewSectionReader(internal.ContextReaderAt(ctx, r), 0, size))
	return orCtxErr(ctx, text, err)
}

// orCtxErr returns ctx's error in place of what an extractor made of a
// cancelled read: a library may stop early without an error, or report what
// the failed read did to its parse rather than why the read failed.
func orCtxErr(ctx context.Context, text []byte, err error) ([]byte, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return text, err
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

func extractDOCXText(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
	doc, err := docx.ReadDocxFromMemory(r, size)
	if err != nil {
		return nil, err
	}
	defer doc.Close()
	return []byte(xmlToText(ctx, strings.NewReader(doc.Editable().GetContent()))), nil
}

func extractPPTXText(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rc, err := s.f.Open()
		if err != nil {
			continue
		}
		sb.WriteString(xmlToText(ctx, rc))
		rc.Close()
	}
	return []byte(sb.String()), nil
}

// ctxCheckEvery is how many units of work, tokens or rows, a converter does
// between looks at ctx: often enough to stop within milliseconds, rarely
// enough to cost nothing.
const ctxCheckEvery = 1024

// xmlToText extracts plain text from XML content, stopping short once ctx is
// done. Paragraph ends (</p> in any namespace) become newlines.
func xmlToText(ctx context.Context, r io.Reader) string {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	for i := 1; ; i++ {
		if i%ctxCheckEvery == 0 && ctx.Err() != nil {
			break
		}
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

// extractXLSXText lists each sheet's rows, tab-separated, as GetRows would
// return them, with the rows a sheet ends on that are empty left out. It
// streams the rows, so it can stop within a sheet once ctx is done: a single
// sheet can take seconds.
//
// Opening the workbook unzips every sheet, which for a large one takes most of
// a second and cannot be interrupted. An open that ctx abandons finishes in
// the background and then closes the workbook, which removes the temporary
// files it unzipped large sheets to, unless the process exits first. lx
// waits for it with WaitCleanups within its grace period after a signal, so
// only an open that outlasts that period leaves its files behind.
func extractXLSXText(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
	f, err := openUntilDone(ctx, &cleanups, func() (*excelize.File, error) {
		return excelize.OpenReader(io.NewSectionReader(r, 0, size))
	})
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		text, err := xlsxSheetText(ctx, f, sheet)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "%s:\n", sheet)
		sb.WriteString(text)
	}
	return []byte(sb.String()), nil
}

// xlsxSheetText renders one sheet's rows. A row that fails to read ends the
// sheet, as it ends GetRows.
func xlsxSheetText(ctx context.Context, f *excelize.File, sheet string) (string, error) {
	rows, err := f.Rows(sheet)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	// Empty rows are written only once a row with cells follows them.
	cur, written := 0, 0
	for rows.Next() {
		cur++
		if cur%ctxCheckEvery == 0 && ctx.Err() != nil {
			break
		}
		row, err := rows.Columns()
		if err != nil {
			break
		}
		if len(row) == 0 {
			continue
		}
		for ; written < cur-1; written++ {
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteByte('\n')
		written = cur
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	return sb.String(), nil
}
