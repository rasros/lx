// Package testzip builds the zip archives tests read, including the minimal
// Office documents that are zip archives underneath.
package testzip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Bytes returns a zip archive of entries, each a name and its content, in
// the order given.
func Bytes(t testing.TB, entries ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// DOCX returns a Word document holding one plain paragraph per paragraphs.
func DOCX(t testing.TB, paragraphs ...string) []byte {
	t.Helper()
	var doc strings.Builder
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	doc.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	doc.WriteString(`<w:body>`)
	for _, p := range paragraphs {
		fmt.Fprintf(&doc, `<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, p)
	}
	doc.WriteString(`</w:body></w:document>`)
	return Bytes(t,
		[2]string{"word/document.xml", doc.String()},
		[2]string{"word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`},
	)
}
