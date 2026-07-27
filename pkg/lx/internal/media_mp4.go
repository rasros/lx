package internal

import (
	"encoding/binary"
	"strings"
)

var mp4Codecs = map[string]string{
	"avc1": "h264", "avc3": "h264", "hev1": "hevc", "hvc1": "hevc",
	"av01": "av1", "vp08": "vp8", "vp09": "vp9", "mp4v": "mpeg4",
	"s263": "h263", "jpeg": "mjpeg",
	"mp4a": "aac", "alac": "alac", "ac-3": "ac3", "ec-3": "eac3",
	"Opus": "opus", ".mp3": "mp3", "fLaC": "flac",
	"twos": "pcm", "sowt": "pcm", "lpcm": "pcm",
}

var losslessAudio = map[string]bool{"pcm": true, "alac": true, "flac": true}

// boxes visits every box between off and end, passing the range of its payload.
func (b *byteReader) boxes(off, end int64, fn func(typ string, start, stop int64)) {
	for off+8 <= end {
		hdr, ok := b.at(off, 8)
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
			ext, ok := b.at(start, 8)
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

func parseMP4(b *byteReader) *MediaInfo {
	if !b.magic(4, "ftyp") {
		return nil
	}

	info := &MediaInfo{}
	found := false
	b.boxes(0, b.size, func(typ string, start, stop int64) {
		switch typ {
		case "moov":
			found = true
			b.boxes(start, stop, func(typ string, start, stop int64) {
				switch typ {
				case "mvhd":
					timescale, duration := b.timedHeader(start, 0)
					info.Duration = secondsToDuration(duration, uint64(timescale))
				case "trak":
					b.track(start, stop, info)
				}
			})
		case "meta":
			// A still image in an ISO container carries no track, only a property
			// describing its extent. The box is a FullBox, so its children start
			// after the version and flags.
			if img := b.imageProperties(start+4, stop); img != nil {
				info.Image = img
				found = true
			}
		}
	})

	if !found {
		return nil
	}
	return info
}

// timedHeader reads the timescale and duration shared by mvhd and mdhd, whose
// layouts differ only in how wide a v1 header makes the timestamps before them.
func (b *byteReader) timedHeader(start, extra int64) (timescale uint32, duration uint64) {
	head, ok := b.at(start, 1)
	if !ok {
		return 0, 0
	}
	if head[0] == 0 {
		p, ok := b.at(start+12+extra, 8)
		if !ok {
			return 0, 0
		}
		return binary.BigEndian.Uint32(p[:4]), uint64(binary.BigEndian.Uint32(p[4:8]))
	}
	p, ok := b.at(start+20+extra, 12)
	if !ok {
		return 0, 0
	}
	return binary.BigEndian.Uint32(p[:4]), binary.BigEndian.Uint64(p[4:12])
}

func (b *byteReader) track(start, stop int64, info *MediaInfo) {
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

	b.boxes(start, stop, func(typ string, s, e int64) {
		switch typ {
		case "tkhd":
			trackWidth, trackHeight = b.trackDimensions(s)
		case "mdia":
			b.boxes(s, e, func(typ string, s, e int64) {
				switch typ {
				case "hdlr":
					if h, ok := b.at(s+8, 4); ok {
						handler = string(h)
					}
				case "mdhd":
					timescale, duration = b.timedHeader(s, 0)
				case "minf":
					b.boxes(s, e, func(typ string, s, e int64) {
						if typ != "stbl" {
							return
						}
						b.boxes(s, e, func(typ string, s, e int64) {
							switch typ {
							case "stsd":
								codec, sampleWidth, sampleHeight,
									channels, sampleRate, bitDepth = b.sampleDescription(s, e)
							case "stts":
								samples = b.sampleCount(s, e)
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
		if d := secondsToDuration(duration, uint64(timescale)); d > 0 && samples > 0 {
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
func (b *byteReader) trackDimensions(start int64) (width, height int) {
	head, ok := b.at(start, 1)
	if !ok {
		return 0, 0
	}
	offset := int64(76)
	if head[0] != 0 {
		offset = 88
	}
	p, ok := b.at(start+offset, 8)
	if !ok {
		return 0, 0
	}
	return int(binary.BigEndian.Uint32(p[:4]) >> 16), int(binary.BigEndian.Uint32(p[4:8]) >> 16)
}

// sampleDescription reads the first sample entry, which names the codec and, for
// audio, carries the channel layout the track was written with.
func (b *byteReader) sampleDescription(start, stop int64) (codec string, width, height, channels, sampleRate, bitDepth int) {
	entry, ok := b.at(start+8, 8)
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
	if body, ok := b.at(base+16, 20); ok && base+36 <= stop {
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
func (b *byteReader) sampleCount(start, stop int64) uint64 {
	p, ok := b.at(start, 8)
	if !ok {
		return 0
	}
	entries := int64(binary.BigEndian.Uint32(p[4:8]))
	if entries <= 0 || start+8+entries*8 > stop {
		return 0
	}
	table, ok := b.at(start+8, entries*8)
	if !ok {
		return 0
	}
	var total uint64
	for i := int64(0); i < entries; i++ {
		total += uint64(binary.BigEndian.Uint32(table[i*8 : i*8+4]))
	}
	return total
}

// imageProperties finds the ispe box that an AVIF or HEIF still stores its
// dimensions in, nested inside the item property container.
func (b *byteReader) imageProperties(start, stop int64) *ImageInfo {
	var img *ImageInfo
	b.boxes(start, stop, func(typ string, s, e int64) {
		if typ != "iprp" {
			return
		}
		b.boxes(s, e, func(typ string, s, e int64) {
			if typ != "ipco" {
				return
			}
			b.boxes(s, e, func(typ string, s, e int64) {
				if typ != "ispe" || img != nil {
					return
				}
				if p, ok := b.at(s+4, 8); ok {
					img = &ImageInfo{
						Width:  int(binary.BigEndian.Uint32(p[:4])),
						Height: int(binary.BigEndian.Uint32(p[4:8])),
					}
				}
			})
		})
	})
	return img
}
