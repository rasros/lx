package internal

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path"
	"strings"
)

// IsImage reports whether the path's extension names an image format. It
// judges the name alone; Classify is what decides once the content is at hand.
func IsImage(p string) bool {
	return FormatByExt(p).IsImage()
}

// MIMEType guesses a content type from the path's extension. The registry
// answers first, so the result does not depend on the host's MIME table; the
// system table covers everything else.
func MIMEType(p string) string {
	if f := FormatByExt(p); f != nil {
		return f.MIME
	}
	if t := mime.TypeByExtension(strings.ToLower(path.Ext(p))); t != "" {
		return t
	}
	return "application/octet-stream"
}

// DataURI encodes already-read bytes. Callers pass content from the source that
// produced the file, so archive entries and URLs work the same as local paths.
// The type comes from the bytes where they identify a format, and from the
// name otherwise.
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
