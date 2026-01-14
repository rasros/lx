package detect

import (
	"path/filepath"
	"strings"
)

var imageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".webp": true,
	".bmp":  true,
	".ico":  true,
	".tiff": true,
	".avif": true,
}

func IsImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExtensions[ext]
}
