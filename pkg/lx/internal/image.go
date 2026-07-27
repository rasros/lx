package internal

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path"
	"strings"
)

// SVG is deliberately absent: it is XML, and reads better as text than as an
// embedded blob. MIMEType still knows it, for templates that pass an svg path
// to dataURI on purpose.
var imageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".ico":  true,
	".tiff": true,
	".avif": true,
}

func IsImage(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return imageExtensions[ext]
}

// MIMEType guesses a content type from the path's extension, covering the
// image types the system table commonly omits.
func MIMEType(p string) string {
	ext := strings.ToLower(path.Ext(p))
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	switch ext {
	case ".svg":
		return "image/svg+xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

// DataURI encodes already-read bytes. Callers pass content from the source that
// produced the file, so archive entries and URLs work the same as local paths.
func DataURI(p string, data []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", MIMEType(p), base64.StdEncoding.EncodeToString(data))
}
