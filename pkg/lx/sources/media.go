package sources

import (
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
	".wav":  "wav",
	".flac": "flac",
}

// IsMediaPath reports whether the path names an audio or video container whose
// metadata can be described.
func IsMediaPath(path string) bool {
	_, ok := mediaSuffixes[strings.ToLower(filepath.Ext(path))]
	return ok
}

// ExtractMediaMetadata describes a media file's container, duration and streams.
// Media is otherwise skipped as binary, so the point is that the file's
// existence and shape reach the bundle at all; an unparsable file still reports
// its container rather than failing.
func ExtractMediaMetadata(f InputFile, r io.ReaderAt, size int64) ([]byte, error) {
	container := mediaSuffixes[strings.ToLower(filepath.Ext(f.Path))]
	return internal.MediaMetadata(container, r, size), nil
}
