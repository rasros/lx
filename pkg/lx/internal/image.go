package internal

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path"
	"strings"
)

// MIMEType guesses a content type from the path's extension, preferring the
// registry over the host's MIME table.
func MIMEType(p string) string {
	if f := FormatByExt(p); f != nil {
		return f.MIME
	}
	if t := mime.TypeByExtension(strings.ToLower(path.Ext(p))); t != "" {
		return t
	}
	return "application/octet-stream"
}

// DataURI encodes already-read bytes, typed by their content or else by the
// name.
func DataURI(p string, data []byte) string {
	mimeType := MIMEType(p)
	header := data
	if len(header) > SniffLen {
		header = header[:SniffLen]
	}
	if f := Classify(p, header); f != nil {
		mimeType = f.MIME
	}
	return DataURIWithType(mimeType, data)
}

// DataURIWithType encodes bytes under a content type the caller already knows.
func DataURIWithType(mimeType string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}
