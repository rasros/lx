package internal

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// MediaInfo is what a media container reveals without decoding. Every field
// is optional.
type MediaInfo struct {
	Container string
	Duration  time.Duration
	// Estimated marks a duration derived from the bitrate rather than read from
	// the container.
	Estimated bool
	Bitrate   int64
	Video     []VideoStream
	Audio     []AudioStream
	Image     *ImageInfo
}

type VideoStream struct {
	Codec         string
	Width, Height int
	FrameRate     float64
}

type AudioStream struct {
	Codec      string
	SampleRate int
	Channels   int
	BitDepth   int
}

type ImageInfo struct {
	Width, Height int
	ColorModel    string
	BitDepth      int
	// Frames counts an animation's frames; zero for a still image.
	Frames int
}

// MediaMetadata describes a media file as a block of text. An empty container
// is sniffed from the header. It reports false only when neither the name nor
// the bytes say media. A stream is read only as far as the parser walks,
// discarding the gaps.
func MediaMetadata(container string, r io.ReaderAt, size int64) ([]byte, bool) {
	b := &byteReader{r: r, size: size}

	info := parseMedia(container, b)
	if info == nil {
		// Either there was no name to go on, or it was wrong. The bytes are not.
		if sniffed := sniffContainer(b); sniffed != "" && sniffed != container {
			if info = parseMedia(sniffed, b); info != nil {
				container = sniffed
			}
		}
	}
	if info == nil {
		if container == "" {
			return nil, false
		}
		info = &MediaInfo{}
	}
	info.Container = container
	// An animation has a duration, but a bitrate is a property of a stream and
	// reads as noise on a picture.
	if info.Bitrate == 0 && info.Duration > 0 && info.Image == nil {
		info.Bitrate = int64(float64(size*8) / info.Duration.Seconds())
	}
	return []byte(info.render()), true
}

// mediaSeeksBack names the containers whose parsers may seek backwards, as a
// TIFF directory points back at its values. Other parsers walk forward.
var mediaSeeksBack = map[string]bool{"tiff": true}

func parseMedia(container string, b *byteReader) *MediaInfo {
	f := FormatByName(container)
	if f == nil || f.parse == nil {
		return nil
	}
	if lr, ok := b.r.(*LazyReader); ok && !mediaSeeksBack[f.Name] {
		defer lr.skipGaps()()
	}
	return f.parse(b)
}

// mediaReadAhead is how much byteReader reads at once, so many small parser
// reads share one source read.
const mediaReadAhead = 64 << 10

// byteReader bounds every read to the file, so a lying header only cuts a
// parse short. It reads ahead.
type byteReader struct {
	r    io.ReaderAt
	size int64
	// buf holds the bytes from bufOff read ahead. It is never written to after
	// it is read, so slices handed out of it stay valid.
	buf    []byte
	bufOff int64
}

// at returns n bytes from off. The slice may share memory with others at
// returns and must not be modified.
func (b *byteReader) at(off, n int64) ([]byte, bool) {
	if off < 0 || n < 0 || off+n > b.size {
		return nil, false
	}
	if off >= b.bufOff && off+n <= b.bufOff+int64(len(b.buf)) {
		i := off - b.bufOff
		return b.buf[i : i+n : i+n], true
	}
	if n > mediaReadAhead {
		// n comes from the file, so it is allocated only as the bytes arrive.
		buf, err := io.ReadAll(io.NewSectionReader(b.r, off, n))
		if err != nil || int64(len(buf)) < n {
			return nil, false
		}
		return buf, true
	}
	buf := make([]byte, min(mediaReadAhead, b.size-off))
	got, _ := b.r.ReadAt(buf, off)
	if int64(got) < n {
		return nil, false
	}
	b.buf, b.bufOff = buf[:got], off
	return buf[:n:n], true
}

func (b *byteReader) magic(off int64, want string) bool {
	got, ok := b.at(off, int64(len(want)))
	return ok && string(got) == want
}

func (i *MediaInfo) render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Container: %s\n", i.Container)

	if i.Duration > 0 {
		fmt.Fprintf(&sb, "Duration: %s", formatDuration(i.Duration))
		if i.Estimated {
			sb.WriteString(" (estimated)")
		}
		sb.WriteString("\n")
	}
	if i.Bitrate > 0 {
		fmt.Fprintf(&sb, "Bitrate: %s\n", formatBitrate(i.Bitrate))
	}

	for _, v := range i.Video {
		parts := []string{v.Codec}
		if v.Width > 0 && v.Height > 0 {
			parts = append(parts, fmt.Sprintf("%dx%d", v.Width, v.Height))
		}
		if v.FrameRate > 0 {
			parts = append(parts, fmt.Sprintf("%.2f fps", v.FrameRate))
		}
		fmt.Fprintf(&sb, "Video: %s\n", joinParts(parts))
	}

	for _, a := range i.Audio {
		parts := []string{a.Codec}
		if a.SampleRate > 0 {
			parts = append(parts, fmt.Sprintf("%d Hz", a.SampleRate))
		}
		if a.Channels > 0 {
			parts = append(parts, formatChannels(a.Channels))
		}
		if a.BitDepth > 0 {
			parts = append(parts, fmt.Sprintf("%d-bit", a.BitDepth))
		}
		fmt.Fprintf(&sb, "Audio: %s\n", joinParts(parts))
	}

	if img := i.Image; img != nil {
		var parts []string
		if img.Width > 0 && img.Height > 0 {
			parts = append(parts, fmt.Sprintf("%dx%d", img.Width, img.Height))
		}
		parts = append(parts, img.ColorModel)
		if img.BitDepth > 0 {
			parts = append(parts, fmt.Sprintf("%d-bit", img.BitDepth))
		}
		if img.Frames > 1 {
			parts = append(parts, fmt.Sprintf("%d frames", img.Frames))
		}
		if joined := joinParts(parts); joined != "" {
			fmt.Fprintf(&sb, "Image: %s\n", joined)
		}
	}

	return sb.String()
}

func joinParts(parts []string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

// formatDuration keeps a fixed shape so that a one-second fixture and a
// three-hour film read the same way.
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

func formatBitrate(bps int64) string {
	switch {
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1e6)
	case bps >= 1000:
		return fmt.Sprintf("%d kbps", bps/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}

func formatChannels(n int) string {
	switch n {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	}
	return fmt.Sprintf("%d channels", n)
}

// secondsToDuration converts units at perSecond units per second.
func secondsToDuration(units uint64, perSecond uint64) time.Duration {
	if perSecond == 0 {
		return 0
	}
	return time.Duration(float64(units) / float64(perSecond) * float64(time.Second))
}
