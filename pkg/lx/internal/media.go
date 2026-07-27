package internal

import (
	"encoding/binary"
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

// MediaMetadata describes a media file as a block of text, so that files which
// would otherwise be skipped as binary contribute their existence and shape to
// the bundle. It never fails: an unparsable file still reports its container.
func MediaMetadata(container string, r io.ReaderAt, size int64) []byte {
	info := parseMedia(container, r, size)
	if info == nil {
		info = &MediaInfo{Container: container}
	}
	info.Container = container
	if info.Bitrate == 0 && info.Duration > 0 {
		info.Bitrate = int64(float64(size*8) / info.Duration.Seconds())
	}
	return []byte(info.render())
}

func parseMedia(container string, r io.ReaderAt, size int64) *MediaInfo {
	switch container {
	case "mp4", "m4a", "mov":
		return parseMP4(r, size)
	case "wav":
		return parseWAV(r, size)
	case "flac":
		return parseFLAC(r, size)
	}
	return nil
}

func (i *MediaInfo) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Container: %s\n", i.Container)
	if i.Duration > 0 {
		fmt.Fprintf(&b, "Duration: %s", formatDuration(i.Duration))
		if i.Estimated {
			b.WriteString(" (estimated)")
		}
		b.WriteString("\n")
	}
	if i.Bitrate > 0 {
		fmt.Fprintf(&b, "Bitrate: %s\n", formatBitrate(i.Bitrate))
	}
	for _, v := range i.Video {
		parts := []string{v.Codec}
		if v.Width > 0 && v.Height > 0 {
			parts = append(parts, fmt.Sprintf("%dx%d", v.Width, v.Height))
		}
		if v.FrameRate > 0 {
			parts = append(parts, fmt.Sprintf("%.2f fps", v.FrameRate))
		}
		fmt.Fprintf(&b, "Video: %s\n", strings.Join(nonEmpty(parts), ", "))
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
		fmt.Fprintf(&b, "Audio: %s\n", strings.Join(nonEmpty(parts), ", "))
	}
	return b.String()
}

func nonEmpty(parts []string) []string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return kept
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

// --- ISO base media format (mp4, m4a, mov) ---

var mp4Codecs = map[string]string{
	"avc1": "h264", "avc3": "h264", "hev1": "hevc", "hvc1": "hevc",
	"av01": "av1", "vp08": "vp8", "vp09": "vp9", "mp4v": "mpeg4",
	"s263": "h263", "jpeg": "mjpeg",
	"mp4a": "aac", "alac": "alac", "ac-3": "ac3", "ec-3": "eac3",
	"Opus": "opus", ".mp3": "mp3", "fLaC": "flac",
	"twos": "pcm", "sowt": "pcm", "lpcm": "pcm",
}

var losslessAudio = map[string]bool{"pcm": true, "alac": true, "flac": true}

type mp4Reader struct {
	r    io.ReaderAt
	size int64
}

func (m *mp4Reader) at(off, n int64) ([]byte, bool) {
	if off < 0 || n < 0 || off+n > m.size {
		return nil, false
	}
	buf := make([]byte, n)
	if _, err := m.r.ReadAt(buf, off); err != nil {
		return nil, false
	}
	return buf, true
}

// boxes visits every box between off and end, passing the range of its payload.
func (m *mp4Reader) boxes(off, end int64, fn func(typ string, start, stop int64)) {
	for off+8 <= end {
		hdr, ok := m.at(off, 8)
		if !ok {
			return
		}
		size := int64(binary.BigEndian.Uint32(hdr[:4]))
		typ := string(hdr[4:8])
		start := off + 8

		switch size {
		case 0: // extends to the end of its parent
			size = end - off
		case 1: // 64-bit size follows the header
			ext, ok := m.at(start, 8)
			if !ok {
				return
			}
			size = int64(binary.BigEndian.Uint64(ext))
			start += 8
		}
		if size < start-off || off+size > end {
			return
		}

		fn(typ, start, off+size)
		off += size
	}
}

func parseMP4(r io.ReaderAt, size int64) *MediaInfo {
	m := &mp4Reader{r: r, size: size}
	if hdr, ok := m.at(4, 4); !ok || string(hdr) != "ftyp" {
		return nil
	}

	info := &MediaInfo{}
	found := false
	m.boxes(0, size, func(typ string, start, stop int64) {
		if typ != "moov" {
			return
		}
		found = true
		m.boxes(start, stop, func(typ string, start, stop int64) {
			switch typ {
			case "mvhd":
				timescale, duration := m.timedHeader(start, stop, 0)
				if timescale > 0 {
					info.Duration = scaledDuration(duration, timescale)
				}
			case "trak":
				m.track(start, stop, info)
			}
		})
	})
	if !found {
		return nil
	}
	return info
}

// timedHeader reads the timescale and duration shared by mvhd and mdhd, whose
// layouts differ only in where the extra field of a v1 header pushes them.
func (m *mp4Reader) timedHeader(start, stop, extra int64) (timescale uint32, duration uint64) {
	head, ok := m.at(start, 1)
	if !ok {
		return 0, 0
	}
	if head[0] == 0 {
		p, ok := m.at(start+12+extra, 8)
		if !ok {
			return 0, 0
		}
		return binary.BigEndian.Uint32(p[:4]), uint64(binary.BigEndian.Uint32(p[4:8]))
	}
	p, ok := m.at(start+20+extra, 12)
	if !ok {
		return 0, 0
	}
	return binary.BigEndian.Uint32(p[:4]), binary.BigEndian.Uint64(p[4:12])
}

func scaledDuration(duration uint64, timescale uint32) time.Duration {
	if timescale == 0 {
		return 0
	}
	return time.Duration(float64(duration) / float64(timescale) * float64(time.Second))
}

func (m *mp4Reader) track(start, stop int64, info *MediaInfo) {
	var (
		handler                   string
		codec                     string
		trackWidth, trackHeight   int
		sampleWidth, sampleHeight int
		channels, sampleRate      int
		bitDepth                  int
		timescale                 uint32
		duration                  uint64
		samples                   uint64
	)

	m.boxes(start, stop, func(typ string, s, e int64) {
		switch typ {
		case "tkhd":
			trackWidth, trackHeight = m.trackDimensions(s)
		case "mdia":
			m.boxes(s, e, func(typ string, s, e int64) {
				switch typ {
				case "hdlr":
					if h, ok := m.at(s+8, 4); ok {
						handler = string(h)
					}
				case "mdhd":
					timescale, duration = m.timedHeader(s, e, 0)
				case "minf":
					m.boxes(s, e, func(typ string, s, e int64) {
						if typ != "stbl" {
							return
						}
						m.boxes(s, e, func(typ string, s, e int64) {
							switch typ {
							case "stsd":
								codec, sampleWidth, sampleHeight,
									channels, sampleRate, bitDepth = m.sampleDescription(s, e)
							case "stts":
								samples = m.sampleCount(s, e)
							}
						})
					})
				}
			})
		}
	})

	switch handler {
	case "vide":
		width, height := sampleWidth, sampleHeight
		if width == 0 || height == 0 {
			width, height = trackWidth, trackHeight
		}
		fps := 0.0
		if d := scaledDuration(duration, timescale); d > 0 && samples > 0 {
			fps = float64(samples) / d.Seconds()
		}
		info.Video = append(info.Video, VideoStream{
			Codec: codec, Width: width, Height: height, FrameRate: fps,
		})
	case "soun":
		if !losslessAudio[codec] {
			// A sample entry's samplesize is a legacy field, fixed at 16 whatever the
			// codec actually does, so it means nothing for a compressed stream.
			bitDepth = 0
		}
		info.Audio = append(info.Audio, AudioStream{
			Codec: codec, SampleRate: sampleRate, Channels: channels, BitDepth: bitDepth,
		})
	}
}

// trackDimensions reads tkhd's display size, stored as 16.16 fixed point after a
// transformation matrix.
func (m *mp4Reader) trackDimensions(start int64) (width, height int) {
	head, ok := m.at(start, 1)
	if !ok {
		return 0, 0
	}
	offset := int64(76)
	if head[0] != 0 {
		offset = 88
	}
	p, ok := m.at(start+offset, 8)
	if !ok {
		return 0, 0
	}
	return int(binary.BigEndian.Uint32(p[:4]) >> 16), int(binary.BigEndian.Uint32(p[4:8]) >> 16)
}

// sampleDescription reads the first sample entry, which names the codec and, for
// audio, carries the channel layout the track was written with.
func (m *mp4Reader) sampleDescription(start, stop int64) (codec string, width, height, channels, sampleRate, bitDepth int) {
	entry, ok := m.at(start+8, 8)
	if !ok {
		return "", 0, 0, 0, 0, 0
	}
	fourcc := string(entry[4:8])
	if mapped, known := mp4Codecs[fourcc]; known {
		codec = mapped
	} else {
		codec = strings.TrimSpace(fourcc)
	}

	base := start + 8
	if body, ok := m.at(base+16, 20); ok && base+36 <= stop {
		// The two entry layouts overlap: audio stores its channel count and rate
		// where video stores dimensions, so which one is meaningful depends on the
		// codec, not on the bytes.
		channels = int(binary.BigEndian.Uint16(body[8:10]))
		bitDepth = int(binary.BigEndian.Uint16(body[10:12]))
		sampleRate = int(binary.BigEndian.Uint32(body[16:20]) >> 16)
		width = int(binary.BigEndian.Uint16(body[16:18]))
		height = int(binary.BigEndian.Uint16(body[18:20]))
	}
	return codec, width, height, channels, sampleRate, bitDepth
}

// sampleCount totals stts's per-run sample counts, which is the frame count.
func (m *mp4Reader) sampleCount(start, stop int64) uint64 {
	p, ok := m.at(start, 8)
	if !ok {
		return 0
	}
	entries := int64(binary.BigEndian.Uint32(p[4:8]))
	if entries <= 0 || start+8+entries*8 > stop {
		return 0
	}
	table, ok := m.at(start+8, entries*8)
	if !ok {
		return 0
	}
	var total uint64
	for i := int64(0); i < entries; i++ {
		total += uint64(binary.BigEndian.Uint32(table[i*8 : i*8+4]))
	}
	return total
}

// --- RIFF (wav) ---

var wavCodecs = map[uint16]string{
	3: "pcm_f32le", 6: "pcm_alaw", 7: "pcm_mulaw", 0x0055: "mp3",
}

func parseWAV(r io.ReaderAt, size int64) *MediaInfo {
	m := &mp4Reader{r: r, size: size}
	hdr, ok := m.at(0, 12)
	if !ok || string(hdr[:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return nil
	}

	var (
		stream   AudioStream
		byteRate uint32
		dataSize int64 = -1
	)

	for off := int64(12); off+8 <= size; {
		head, ok := m.at(off, 8)
		if !ok {
			break
		}
		id := string(head[:4])
		length := int64(binary.LittleEndian.Uint32(head[4:8]))
		body := off + 8

		switch id {
		case "fmt ":
			if p, ok := m.at(body, 16); ok {
				format := binary.LittleEndian.Uint16(p[:2])
				stream.Codec = wavCodecs[format]
				stream.Channels = int(binary.LittleEndian.Uint16(p[2:4]))
				stream.SampleRate = int(binary.LittleEndian.Uint32(p[4:8]))
				stream.BitDepth = int(binary.LittleEndian.Uint16(p[14:16]))
				byteRate = binary.LittleEndian.Uint32(p[8:12])
				if stream.Codec == "" {
					stream.Codec = fmt.Sprintf("pcm_s%dle", stream.BitDepth)
				}
			}
		case "data":
			dataSize = length
		}

		off = body + length + length%2 // chunks are padded to an even length
	}

	if stream.Codec == "" {
		return nil
	}
	info := &MediaInfo{Audio: []AudioStream{stream}}
	if dataSize > 0 && byteRate > 0 {
		info.Duration = time.Duration(float64(dataSize) / float64(byteRate) * float64(time.Second))
	}
	return info
}

// --- FLAC ---

func parseFLAC(r io.ReaderAt, size int64) *MediaInfo {
	m := &mp4Reader{r: r, size: size}
	if magic, ok := m.at(0, 4); !ok || string(magic) != "fLaC" {
		return nil
	}
	// STREAMINFO is mandatory and always first, so there is no need to walk the
	// remaining metadata blocks.
	p, ok := m.at(8, 18)
	if !ok {
		return nil
	}

	packed := binary.BigEndian.Uint64(p[10:18])
	sampleRate := int(packed >> 44)
	stream := AudioStream{
		Codec:      "flac",
		SampleRate: sampleRate,
		Channels:   int(packed>>41&0x7) + 1,
		BitDepth:   int(packed>>36&0x1f) + 1,
	}
	info := &MediaInfo{Audio: []AudioStream{stream}}
	if total := packed & 0xf_ffff_ffff; total > 0 && sampleRate > 0 {
		info.Duration = time.Duration(float64(total) / float64(sampleRate) * float64(time.Second))
	}
	return info
}
