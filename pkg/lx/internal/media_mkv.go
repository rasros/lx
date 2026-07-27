package internal

import (
	"encoding/binary"
	"math"
	"strings"
)

// Matroska element IDs, kept as the values they are stored as, marker bits and
// all, so they can be compared without decoding.
const (
	ebmlHeader    = 0x1a45dfa3
	mkvSegment    = 0x18538067
	mkvInfo       = 0x1549a966
	mkvTimeScale  = 0x2ad7b1
	mkvDuration   = 0x4489
	mkvTracks     = 0x1654ae6b
	mkvTrackEntry = 0xae
	mkvTrackType  = 0x83
	mkvCodecID    = 0x86
	mkvVideo      = 0xe0
	mkvPixelWidth = 0xb0
	mkvPixelHeigh = 0xba
	mkvFrameTime  = 0x23e383
	mkvAudio      = 0xe1
	mkvSampleRate = 0xb5
	mkvChannels   = 0x9f
	mkvBitDepth   = 0x6264
)

// mkvCodecs maps the codec strings Matroska uses onto the names the rest of the
// output uses, so an mkv and an mp4 holding the same stream describe it alike.
var mkvCodecs = map[string]string{
	"V_MPEG4/ISO/AVC": "h264", "V_MPEGH/ISO/HEVC": "hevc", "V_AV1": "av1",
	"V_VP8": "vp8", "V_VP9": "vp9", "V_MPEG4/ISO/ASP": "mpeg4",
	"V_MPEG2": "mpeg2", "V_THEORA": "theora",
	"A_AAC": "aac", "A_OPUS": "opus", "A_VORBIS": "vorbis", "A_FLAC": "flac",
	"A_AC3": "ac3", "A_EAC3": "eac3", "A_TRUEHD": "truehd", "A_DTS": "dts",
	"A_MPEG/L3": "mp3", "A_MPEG/L2": "mp2", "A_ALAC": "alac",
}

// The default when a segment omits it: timestamps are in milliseconds.
const mkvDefaultTimeScale = 1_000_000

func parseMatroska(b *byteReader) *MediaInfo {
	id, _, next, ok := b.ebmlElement(0)
	if !ok || id != ebmlHeader {
		return nil
	}

	info := &MediaInfo{}
	found := false
	b.ebmlChildren(next, b.size, func(id uint64, start, stop int64) {
		if id != mkvSegment {
			return
		}
		found = true

		timeScale := uint64(mkvDefaultTimeScale)
		var scaledDuration float64
		b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
			switch id {
			case mkvInfo:
				b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
					switch id {
					case mkvTimeScale:
						if v, ok := b.ebmlUint(start, stop); ok && v > 0 {
							timeScale = v
						}
					case mkvDuration:
						scaledDuration, _ = b.ebmlFloat(start, stop)
					}
				})
			case mkvTracks:
				b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
					if id == mkvTrackEntry {
						b.mkvTrack(start, stop, info)
					}
				})
			}
		})

		// Duration is a count of ticks, and the scale says how many nanoseconds
		// each tick is worth.
		if scaledDuration > 0 {
			info.Duration = secondsToDuration(
				uint64(scaledDuration*float64(timeScale)), uint64(1e9))
		}
	})

	if !found {
		return nil
	}
	return info
}

func (b *byteReader) mkvTrack(start, stop int64, info *MediaInfo) {
	var (
		trackType          uint64
		codec              string
		width, height      int
		frameTime          uint64
		sampleRate         int
		channels, bitDepth int
	)

	b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
		switch id {
		case mkvTrackType:
			trackType, _ = b.ebmlUint(start, stop)
		case mkvCodecID:
			raw := strings.TrimRight(string(b.ebmlBytes(start, stop)), "\x00")
			if mapped, known := mkvCodecs[raw]; known {
				codec = mapped
			} else {
				codec = strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(raw, "V_"), "A_"))
			}
		case mkvVideo:
			b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
				switch id {
				case mkvPixelWidth:
					if v, ok := b.ebmlUint(start, stop); ok {
						width = int(v)
					}
				case mkvPixelHeigh:
					if v, ok := b.ebmlUint(start, stop); ok {
						height = int(v)
					}
				case mkvFrameTime:
					frameTime, _ = b.ebmlUint(start, stop)
				}
			})
		case mkvAudio:
			b.ebmlChildren(start, stop, func(id uint64, start, stop int64) {
				switch id {
				case mkvSampleRate:
					if v, ok := b.ebmlFloat(start, stop); ok {
						sampleRate = int(v)
					}
				case mkvChannels:
					if v, ok := b.ebmlUint(start, stop); ok {
						channels = int(v)
					}
				case mkvBitDepth:
					if v, ok := b.ebmlUint(start, stop); ok {
						bitDepth = int(v)
					}
				}
			})
		}
	})

	switch trackType {
	case 1:
		fps := 0.0
		if frameTime > 0 {
			// DefaultDuration is the nanoseconds one frame lasts.
			fps = 1e9 / float64(frameTime)
		}
		info.Video = append(info.Video, VideoStream{
			Codec: codec, Width: width, Height: height, FrameRate: fps,
		})
	case 2:
		if !losslessAudio[codec] {
			// Muxers fill BitDepth in from the decoded sample format, which says
			// nothing about a compressed stream.
			bitDepth = 0
		}
		// SamplingFrequency is what the track declares. For Opus that is the rate
		// the audio was encoded from rather than the 48 kHz it always decodes at,
		// and the header is what is being reported here.
		info.Audio = append(info.Audio, AudioStream{
			Codec: codec, SampleRate: sampleRate, Channels: channels, BitDepth: bitDepth,
		})
	}
}

// ebmlElement reads one element header: an ID that keeps its length marker and a
// size that drops it.
func (b *byteReader) ebmlElement(off int64) (id uint64, start, stop int64, ok bool) {
	id, idLen, ok := b.ebmlNumber(off, true)
	if !ok {
		return 0, 0, 0, false
	}
	size, sizeLen, ok := b.ebmlNumber(off+idLen, false)
	if !ok {
		return 0, 0, 0, false
	}

	start = off + idLen + sizeLen
	// An unknown size, all bits set, means the element runs to the end of what
	// contains it; live-muxed files are written that way.
	if size == ebmlUnknownSize(sizeLen) || start+int64(size) > b.size {
		return id, start, b.size, true
	}
	return id, start, start + int64(size), true
}

func ebmlUnknownSize(sizeLen int64) uint64 {
	return uint64(1)<<(sizeLen*7) - 1
}

// ebmlNumber decodes a variable-length integer, whose width is the number of
// leading zero bits in its first byte.
func (b *byteReader) ebmlNumber(off int64, keepMarker bool) (value uint64, length int64, ok bool) {
	first, ok := b.at(off, 1)
	if !ok || first[0] == 0 {
		return 0, 0, false
	}

	length = int64(1)
	for mask := byte(0x80); first[0]&mask == 0; mask >>= 1 {
		length++
	}
	buf, ok := b.at(off, length)
	if !ok {
		return 0, 0, false
	}

	value = uint64(buf[0])
	if !keepMarker {
		value &= uint64(0xff) >> uint(length) // clear the length marker
	}
	for _, c := range buf[1:] {
		value = value<<8 | uint64(c)
	}
	return value, length, true
}

func (b *byteReader) ebmlChildren(start, stop int64, fn func(id uint64, start, stop int64)) {
	for off := start; off < stop; {
		id, childStart, childStop, ok := b.ebmlElement(off)
		if !ok || childStop <= off || childStop > stop {
			return
		}
		fn(id, childStart, childStop)
		off = childStop
	}
}

func (b *byteReader) ebmlBytes(start, stop int64) []byte {
	if buf, ok := b.at(start, stop-start); ok {
		return buf
	}
	return nil
}

// ebmlUint reads an integer stored in as few bytes as it fits into.
func (b *byteReader) ebmlUint(start, stop int64) (uint64, bool) {
	buf := b.ebmlBytes(start, stop)
	if len(buf) == 0 || len(buf) > 8 {
		return 0, false
	}
	var v uint64
	for _, c := range buf {
		v = v<<8 | uint64(c)
	}
	return v, true
}

func (b *byteReader) ebmlFloat(start, stop int64) (float64, bool) {
	switch buf := b.ebmlBytes(start, stop); len(buf) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(buf))), true
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(buf)), true
	}
	return 0, false
}
