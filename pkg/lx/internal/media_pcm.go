package internal

import (
	"encoding/binary"
	"fmt"
)

var wavCodecs = map[uint16]string{
	3: "pcm_f32le", 6: "pcm_alaw", 7: "pcm_mulaw", 0x0055: "mp3",
}

func parseWAV(b *byteReader) *MediaInfo {
	if !b.magic(0, "RIFF") || !b.magic(8, "WAVE") {
		return nil
	}

	var (
		stream   AudioStream
		byteRate uint32
		dataSize int64
	)

	b.riffChunks(12, func(id string, body, length int64) {
		switch id {
		case "fmt ":
			p, ok := b.at(body, 16)
			if !ok {
				return
			}
			format := binary.LittleEndian.Uint16(p[:2])
			stream.Codec = wavCodecs[format]
			stream.Channels = int(binary.LittleEndian.Uint16(p[2:4]))
			stream.SampleRate = int(binary.LittleEndian.Uint32(p[4:8]))
			stream.BitDepth = int(binary.LittleEndian.Uint16(p[14:16]))
			byteRate = binary.LittleEndian.Uint32(p[8:12])
			if stream.Codec == "" {
				stream.Codec = fmt.Sprintf("pcm_s%dle", stream.BitDepth)
			}
		case "data":
			dataSize = length
		}
	})

	if stream.Codec == "" {
		return nil
	}
	info := &MediaInfo{Audio: []AudioStream{stream}}
	if dataSize > 0 && byteRate > 0 {
		info.Duration = secondsToDuration(uint64(dataSize), uint64(byteRate))
	}
	return info
}

// riffChunks walks a RIFF chunk list, which WAV and WebP both use.
func (b *byteReader) riffChunks(off int64, fn func(id string, body, length int64)) {
	for off+8 <= b.size {
		head, ok := b.at(off, 8)
		if !ok {
			return
		}
		length := int64(binary.LittleEndian.Uint32(head[4:8]))
		if length < 0 {
			return
		}
		fn(string(head[:4]), off+8, length)
		off += 8 + length + length%2 // chunks are padded to an even length
	}
}

func parseFLAC(b *byteReader) *MediaInfo {
	if !b.magic(0, "fLaC") {
		return nil
	}
	// STREAMINFO is mandatory and always first, so there is no need to walk the
	// remaining metadata blocks.
	p, ok := b.at(8, 18)
	if !ok {
		return nil
	}

	packed := binary.BigEndian.Uint64(p[10:18])
	sampleRate := uint64(packed >> 44)
	info := &MediaInfo{Audio: []AudioStream{{
		Codec:      "flac",
		SampleRate: int(sampleRate),
		Channels:   int(packed>>41&0x7) + 1,
		BitDepth:   int(packed>>36&0x1f) + 1,
	}}}
	if total := packed & 0xf_ffff_ffff; total > 0 {
		info.Duration = secondsToDuration(total, sampleRate)
	}
	return info
}
