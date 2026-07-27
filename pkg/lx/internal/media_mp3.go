package internal

import "encoding/binary"

// The bitrate a frame header names depends on both the MPEG version and the
// layer, so the tables are indexed by the four bits the header carries.
var mp3Bitrates = map[int][15]int{
	// MPEG 1, layers I, II and III.
	11: {0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448},
	12: {0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384},
	13: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320},
	// MPEG 2 and 2.5, whose layers II and III share a table.
	21: {0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256},
	22: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},
}

var mp3SampleRates = map[int][3]int{
	1:  {44100, 48000, 32000}, // MPEG 1
	2:  {22050, 24000, 16000}, // MPEG 2
	25: {11025, 12000, 8000},  // MPEG 2.5
}

var mp3LayerCodecs = map[int]string{1: "mp1", 2: "mp2", 3: "mp3"}

// mp3FrameSearch bounds how far into a file a first frame is looked for, so a
// file that merely ends in .mp3 costs a single read rather than a full scan.
const mp3FrameSearch = 64 << 10

func parseMP3(b *byteReader) *MediaInfo {
	start := b.id3Size()
	offset, header, ok := b.findMP3Frame(start)
	if !ok {
		return nil
	}

	version, layer := mp3Version(header), int(header[1]>>1&0x3)
	switch layer {
	case 1:
		layer = 3 // the bits count down: 1 means layer III
	case 3:
		layer = 1
	}

	rates, knownVersion := mp3SampleRates[version]
	bitrates, knownTable := mp3Bitrates[mp3TableKey(version, layer)]
	rateIndex, bitrateIndex := int(header[2]>>2&0x3), int(header[2]>>4&0xf)
	if !knownVersion || !knownTable || rateIndex > 2 || bitrateIndex == 0 || bitrateIndex > 14 {
		return nil
	}

	sampleRate := rates[rateIndex]
	bitrate := bitrates[bitrateIndex] * 1000
	channels := 2
	if header[3]>>6&0x3 == 3 {
		channels = 1
	}

	info := &MediaInfo{
		Bitrate: int64(bitrate),
		Audio: []AudioStream{{
			Codec:      mp3LayerCodecs[layer],
			SampleRate: sampleRate,
			Channels:   channels,
		}},
	}

	// A variable-bitrate file states its frame count in the first frame; without
	// one, the nominal bitrate is all there is to divide the file by.
	if frames, ok := b.vbrFrameCount(offset, version, channels); ok {
		info.Duration = secondsToDuration(frames*uint64(mp3SamplesPerFrame(version, layer)), uint64(sampleRate))
	} else if bitrate > 0 {
		// The nominal bitrate stays as it was read from the header. Deriving one
		// from the estimate instead would only hand back the number that produced
		// it, dressed up as a measurement.
		info.Duration = secondsToDuration(uint64((b.size-offset)*8), uint64(bitrate))
		info.Estimated = true
	}
	return info
}

// id3Size reports where the audio starts, skipping an ID3v2 tag if one is
// present. Its length is stored as four 7-bit groups so that it can never
// contain a byte that looks like a frame sync.
func (b *byteReader) id3Size() int64 {
	if !b.magic(0, "ID3") {
		return 0
	}
	p, ok := b.at(6, 4)
	if !ok {
		return 0
	}
	size := int64(p[0]&0x7f)<<21 | int64(p[1]&0x7f)<<14 | int64(p[2]&0x7f)<<7 | int64(p[3]&0x7f)
	return 10 + size
}

func (b *byteReader) findMP3Frame(start int64) (offset int64, header []byte, ok bool) {
	length := b.size - start
	if length > mp3FrameSearch {
		length = mp3FrameSearch
	}
	buf, ok := b.at(start, length)
	if !ok {
		return 0, nil, false
	}

	for i := 0; i+4 <= len(buf); i++ {
		// A sync is eleven set bits, and the version and layer that follow must
		// both be values that exist.
		if buf[i] != 0xff || buf[i+1]&0xe0 != 0xe0 {
			continue
		}
		if buf[i+1]>>3&0x3 == 1 || buf[i+1]>>1&0x3 == 0 {
			continue
		}
		return start + int64(i), buf[i : i+4], true
	}
	return 0, nil, false
}

func mp3Version(header []byte) int {
	switch header[1] >> 3 & 0x3 {
	case 3:
		return 1
	case 2:
		return 2
	}
	return 25
}

// mp3TableKey folds the version and layer into the bitrate table they share:
// MPEG 2 and 2.5 use one table for layers II and III.
func mp3TableKey(version, layer int) int {
	if version == 1 {
		return 10 + layer
	}
	if layer == 1 {
		return 21
	}
	return 22
}

func mp3SamplesPerFrame(version, layer int) int {
	switch {
	case layer == 1:
		return 384
	case layer == 3 && version != 1:
		return 576
	}
	return 1152
}

// vbrFrameCount reads the Xing or VBRI header a variable-bitrate encoder writes
// into the first frame, past the side information whose size depends on the
// version and channel count.
func (b *byteReader) vbrFrameCount(frame int64, version, channels int) (uint64, bool) {
	sideInfo := int64(32)
	switch {
	case version != 1 && channels == 1:
		sideInfo = 9
	case version != 1:
		sideInfo = 17
	case channels == 1:
		sideInfo = 17
	}

	if tag, ok := b.at(frame+4+sideInfo, 12); ok {
		name := string(tag[:4])
		if name == "Xing" || name == "Info" {
			if binary.BigEndian.Uint32(tag[4:8])&0x1 != 0 {
				return uint64(binary.BigEndian.Uint32(tag[8:12])), true
			}
		}
	}
	// VBRI sits at a fixed offset instead, always after a full side info block.
	if tag, ok := b.at(frame+36, 18); ok && string(tag[:4]) == "VBRI" {
		return uint64(binary.BigEndian.Uint32(tag[14:18])), true
	}
	return 0, false
}
