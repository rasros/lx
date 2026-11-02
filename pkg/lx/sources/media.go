package sources

import (
	"fmt"
	"io"

	"github.com/rasros/lx/pkg/lx/internal"
)

// IsMediaPath reports whether the path's extension names an audio, video or
// image format whose metadata can be described. It judges the name alone;
// ExtractMediaMetadata looks at the content as well.
func IsMediaPath(path string) bool {
	return internal.FormatByExt(path).IsMedia()
}

// ExtractMediaMetadata describes a media file's container, duration and streams.
// Media is otherwise skipped as binary, so the point is that the file's
// existence and shape reach the bundle at all.
//
// It fails for a file that is not media, judged by its content as well as its
// name, which leaves the content untouched for the text or binary handling to
// describe as it would have anyway. Text named like media (a Git LFS pointer,
// say) is not media.
func ExtractMediaMetadata(f InputFile, r io.ReaderAt, size int64) ([]byte, error) {
	n := size
	if n > internal.SniffLen {
		n = internal.SniffLen
	}
	header := make([]byte, max(n, 0))
	read, _ := r.ReadAt(header, 0)
	format := internal.Classify(f.Path, header[:read])
	if !format.IsMedia() {
		return nil, fmt.Errorf("no media container found in %s", f.Path)
	}

	// The container is named only when the name agrees with the content: a name
	// alone vouches for a file that will not parse, and content that was merely
	// sniffed does not.
	container := ""
	if format == internal.FormatByExt(f.Path) {
		container = format.Name
	}
	data, ok := internal.MediaMetadata(container, r, size)
	if !ok {
		return nil, fmt.Errorf("no media container found in %s", f.Path)
	}
	return data, nil
}
