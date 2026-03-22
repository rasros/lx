package detect

import (
	"path"
	"strings"
)

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
