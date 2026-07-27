package sources

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rasros/lx/pkg/lx/internal"
)

// mediaSuffixes maps a file extension to the container it names. Extensions
// decide the parser: sniffing would only distinguish containers the extension
// already names, and a mislabelled file falls back to reporting its container
// either way.
var mediaSuffixes = map[string]string{
	".mp4":  "mp4",
	".m4v":  "mp4",
	".m4a":  "m4a",
	".mov":  "mov",
	".mkv":  "mkv",
	".webm": "webm",
	".mp3":  "mp3",
	".wav":  "wav",
	".flac": "flac",
	".png":  "png",
	".jpg":  "jpeg",
	".jpeg": "jpeg",
	".gif":  "gif",
	".webp": "webp",
	".bmp":  "bmp",
	".ico":  "ico",
	".tif":  "tiff",
	".tiff": "tiff",
	".avif": "avif",
	".heic": "heic",
}

// IsMediaPath reports whether the path names an audio or video container whose
// metadata can be described.
func IsMediaPath(path string) bool {
	_, ok := mediaSuffixes[strings.ToLower(filepath.Ext(path))]
	return ok
}

// IsMediaCandidate reports whether a path is worth offering to media
// extraction. A recognised suffix settles it, and a path with no suffix at all
// is offered too: URLs frequently have none, and the header is what identifies
// those. Extraction rejects what turns out not to be media.
func IsMediaCandidate(path string) bool {
	return IsMediaPath(path) || filepath.Ext(path) == ""
}

// ExtractMediaMetadata describes a media file's container, duration and streams.
// Media is otherwise skipped as binary, so the point is that the file's
// existence and shape reach the bundle at all.
//
// It fails only for a file that is not media, which leaves the content untouched
// for the binary handling to describe as it would have anyway.
func ExtractMediaMetadata(f InputFile, r io.ReaderAt, size int64) ([]byte, error) {
	container := mediaSuffixes[strings.ToLower(filepath.Ext(f.Path))]
	data, ok := internal.MediaMetadata(container, r, size)
	if !ok {
		return nil, fmt.Errorf("no media container found in %s", f.Path)
	}
	return data, nil
}
