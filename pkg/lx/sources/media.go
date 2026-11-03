package sources

import (
	"fmt"
	"io"

	"github.com/rasros/lx/pkg/lx/internal"
)

// ExtractMediaMetadata describes a media file's container, duration and
// streams. It fails for a file whose content is not media, such as a Git LFS
// pointer named like media.
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

	// Name the container only when the name agrees with the content.
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
