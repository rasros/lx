package internal

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// MediaInfo is what a media container reveals about itself without decoding any
// of it. Every field is optional: containers disagree about what they store in
// their headers, and a truncated file may yield only some of it.
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

// MediaMetadata describes a media file as a block of text, so that files which
// would otherwise be skipped as binary contribute their existence and shape to
// the bundle. It never fails: an unparsable file still reports its container.
func MediaMetadata(container string, r io.ReaderAt, size int64) []byte {
	info := parseMedia(container, r, size)
	if info == nil {
		info = &MediaInfo{}
	}
	info.Container = container
	// An animation has a duration, but a bitrate is a property of a stream and
	// reads as noise on a picture.
	if info.Bitrate == 0 && info.Duration > 0 && info.Image == nil {
		info.Bitrate = int64(float64(size*8) / info.Duration.Seconds())
	}
	return []byte(info.render())
}

func parseMedia(container string, r io.ReaderAt, size int64) *MediaInfo {
	b := &byteReader{r: r, size: size}
	switch container {
	// AVIF and HEIC are the same boxes as an mp4, holding a still rather than a
	// track.
	case "mp4", "m4a", "mov", "avif":
		return parseMP4(b)
	case "mkv", "webm":
		return parseMatroska(b)
	case "wav":
		return parseWAV(b)
	case "flac":
		return parseFLAC(b)
	case "mp3":
		return parseMP3(b)
	case "png":
		return parsePNG(b)
	case "jpeg":
		return parseJPEG(b)
	case "gif":
		return parseGIF(b)
	case "webp":
		return parseWebP(b)
	case "bmp":
		return parseBMP(b)
	case "ico":
		return parseICO(b)
	case "tiff":
		return parseTIFF(b)
	}
	return nil
}

// byteReader bounds every read to the file, so a truncated or lying header can
// only cut a parse short rather than run off the end.
type byteReader struct {
	r    io.ReaderAt
	size int64
}

func (b *byteReader) at(off, n int64) ([]byte, bool) {
	if off < 0 || n < 0 || off+n > b.size {
		return nil, false
	}
	buf := make([]byte, n)
	if _, err := b.r.ReadAt(buf, off); err != nil {
		return nil, false
	}
	return buf, true
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

// secondsToDuration converts a count of units and the units per second they are
// measured in, which is how every container here stores its timings.
func secondsToDuration(units uint64, perSecond uint64) time.Duration {
	if perSecond == 0 {
		return 0
	}
	return time.Duration(float64(units) / float64(perSecond) * float64(time.Second))
}
