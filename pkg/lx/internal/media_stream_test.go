package internal

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// --- MP3 ---

// mp3Header builds a frame header. The bits count down, so layer III is 1 and
// MPEG 1 is 3, which is why the builder takes the values rather than the bits.
func mp3Header(version, layer, bitrateIndex, rateIndex, channels int) []byte {
	versionBits := map[int]byte{1: 3, 2: 2, 25: 0}[version]
	layerBits := map[int]byte{1: 3, 2: 2, 3: 1}[layer]

	mode := byte(0) // stereo
	if channels == 1 {
		mode = 3
	}
	return []byte{
		0xff,
		0xe0 | versionBits<<3 | layerBits<<1,
		byte(bitrateIndex)<<4 | byte(rateIndex)<<2,
		mode << 6,
	}
}

// xingFrame builds a first frame carrying a frame count, placed after the side
// information for the version and channel count.
func xingFrame(header []byte, sideInfo int, frames uint32) []byte {
	out := append([]byte{}, header...)
	out = append(out, zeros(sideInfo)...)
	out = append(out, []byte("Xing")...)
	out = append(out, be32(0x1)...) // the frames field is present
	out = append(out, be32(frames)...)
	return out
}

func TestMediaMetadataMP3WithXingHeader(t *testing.T) {
	// 100 frames of 1152 samples at 44100 Hz.
	data := xingFrame(mp3Header(1, 3, 9, 0, 2), 32, 100)
	data = append(data, zeros(4096)...)

	got := describe(t, "mp3", data)
	for _, want := range []string{"Duration: 00:00:02.612", "Bitrate: 128 kbps", "Audio: mp3, 44100 Hz, stereo"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "estimated") {
		t.Errorf("marked a stated duration as estimated:\n%s", got)
	}
}

// Without a frame count there is only the nominal bitrate to divide by, and the
// result has to say so.
func TestMediaMetadataMP3EstimatesWithoutXing(t *testing.T) {
	data := append(mp3Header(1, 3, 9, 0, 2), zeros(16000)...)

	got := describe(t, "mp3", data)
	if !strings.Contains(got, "(estimated)") {
		t.Errorf("did not mark the estimate:\n%s", got)
	}
	// The bitrate is the one the header states, not one derived from the estimate
	// it produced.
	if !strings.Contains(got, "Bitrate: 128 kbps") {
		t.Errorf("lost the stated bitrate:\n%s", got)
	}
}

// A mono frame has half the side information, so the tag sits earlier.
func TestMediaMetadataMP3MonoSideInfo(t *testing.T) {
	data := xingFrame(mp3Header(1, 3, 5, 0, 1), 17, 50)
	data = append(data, zeros(1024)...)

	got := describe(t, "mp3", data)
	if !strings.Contains(got, "mono") || !strings.Contains(got, "Duration: 00:00:01.306") {
		t.Errorf("got:\n%s", got)
	}
}

// MPEG 2 halves the samples per frame and uses its own bitrate table.
func TestMediaMetadataMP3Version2(t *testing.T) {
	data := xingFrame(mp3Header(2, 3, 5, 0, 1), 9, 100)
	data = append(data, zeros(1024)...)

	got := describe(t, "mp3", data)
	for _, want := range []string{"22050 Hz", "Duration: 00:00:02.612"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestMediaMetadataMP3Layer2(t *testing.T) {
	data := append(mp3Header(1, 2, 10, 1, 2), zeros(4096)...)
	if got := describe(t, "mp3", data); !strings.Contains(got, "Audio: mp2, 48000 Hz, stereo") {
		t.Errorf("got:\n%s", got)
	}
}

// An ID3 tag is skipped by the length in its header, which is stored as four
// 7-bit groups so it can never look like a frame sync.
func TestMediaMetadataMP3SkipsID3Tag(t *testing.T) {
	tag := append([]byte("ID3\x04\x00\x00"), []byte{0, 0, 1, 0x7f}...) // 1<<7|127 = 255 bytes
	tag = append(tag, zeros(255)...)

	data := append(tag, xingFrame(mp3Header(1, 3, 9, 0, 2), 32, 100)...)
	data = append(data, zeros(4096)...)

	if got := describe(t, "mp3", data); !strings.Contains(got, "Duration: 00:00:02.612") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataMP3RejectsReservedValues(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"free bitrate", append(mp3Header(1, 3, 0, 0, 2), zeros(64)...)},
		{"reserved bitrate", append(mp3Header(1, 3, 15, 0, 2), zeros(64)...)},
		{"reserved sample rate", append(mp3Header(1, 3, 9, 3, 2), zeros(64)...)},
		{"no sync at all", []byte("not audio, just bytes")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(t, "mp3", c.data); got != "Container: mp3" {
				t.Errorf("got:\n%s", got)
			}
		})
	}
}

// --- Matroska ---

// ebmlID writes an element ID, which is stored with its length marker intact.
func ebmlID(id uint32) []byte {
	switch {
	case id <= 0xff:
		return []byte{byte(id)}
	case id <= 0xffff:
		return be16(uint16(id))
	case id <= 0xffffff:
		return []byte{byte(id >> 16), byte(id >> 8), byte(id)}
	}
	return be32(id)
}

// ebmlElem writes an element with a one-byte size, whose marker is the high bit.
func ebmlElem(id uint32, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	out := ebmlID(id)
	out = append(out, byte(0x80|len(body)))
	return append(out, body...)
}

func ebmlUintElem(id uint32, v uint64) []byte {
	var body []byte
	for shift := 56; shift >= 0; shift -= 8 {
		if b := byte(v >> uint(shift)); b != 0 || len(body) > 0 {
			body = append(body, b)
		}
	}
	if len(body) == 0 {
		body = []byte{0}
	}
	return ebmlElem(id, body)
}

func ebmlFloatElem(id uint32, v float64) []byte {
	return ebmlElem(id, binary.BigEndian.AppendUint64(nil, math.Float64bits(v)))
}

func matroskaFile(parts ...[]byte) []byte {
	header := ebmlElem(ebmlHeader, []byte{0x42, 0x86, 0x81, 0x01})
	return append(header, ebmlElem(mkvSegment, bytes.Join(parts, nil))...)
}

func TestMediaMetadataMatroska(t *testing.T) {
	data := matroskaFile(
		ebmlElem(mkvInfo,
			ebmlUintElem(mkvTimeScale, 1_000_000),
			ebmlFloatElem(mkvDuration, 5000)), // 5000 ms
		ebmlElem(mkvTracks,
			ebmlElem(mkvTrackEntry,
				ebmlUintElem(mkvTrackType, 1),
				ebmlElem(mkvCodecID, []byte("V_MPEG4/ISO/AVC")),
				ebmlElem(mkvVideo,
					ebmlUintElem(mkvPixelWidth, 1920),
					ebmlUintElem(mkvPixelHeigh, 1080),
					ebmlUintElem(mkvFrameTime, 40_000_000))),
			ebmlElem(mkvTrackEntry,
				ebmlUintElem(mkvTrackType, 2),
				ebmlElem(mkvCodecID, []byte("A_OPUS")),
				ebmlElem(mkvAudio,
					ebmlFloatElem(mkvSampleRate, 48000),
					ebmlUintElem(mkvChannels, 2)))),
	)

	got := describe(t, "mkv", data)
	for _, want := range []string{
		"Duration: 00:00:05.000",
		"Video: h264, 1920x1080, 25.00 fps",
		"Audio: opus, 48000 Hz, stereo",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// A timestamp scale other than the usual millisecond has to be applied.
func TestMediaMetadataMatroskaTimeScale(t *testing.T) {
	data := matroskaFile(ebmlElem(mkvInfo,
		ebmlUintElem(mkvTimeScale, 1_000), // ticks of a microsecond
		ebmlFloatElem(mkvDuration, 2_000_000)))

	if got := describe(t, "mkv", data); !strings.Contains(got, "Duration: 00:00:02.000") {
		t.Errorf("got:\n%s", got)
	}
}

// A segment that omits the scale means milliseconds.
func TestMediaMetadataMatroskaDefaultTimeScale(t *testing.T) {
	data := matroskaFile(ebmlElem(mkvInfo, ebmlFloatElem(mkvDuration, 1500)))
	if got := describe(t, "mkv", data); !strings.Contains(got, "Duration: 00:00:01.500") {
		t.Errorf("got:\n%s", got)
	}
}

// Muxers write BitDepth from the decoded sample format, which is meaningless for
// a compressed stream but not for a lossless one.
func TestMediaMetadataMatroskaBitDepth(t *testing.T) {
	audio := func(codec string, depth uint64) []byte {
		return matroskaFile(ebmlElem(mkvTracks, ebmlElem(mkvTrackEntry,
			ebmlUintElem(mkvTrackType, 2),
			ebmlElem(mkvCodecID, []byte(codec)),
			ebmlElem(mkvAudio,
				ebmlFloatElem(mkvSampleRate, 44100),
				ebmlUintElem(mkvChannels, 2),
				ebmlUintElem(mkvBitDepth, depth)))))
	}

	if got := describe(t, "mkv", audio("A_AAC", 32)); strings.Contains(got, "32-bit") {
		t.Errorf("kept a decoded bit depth for aac:\n%s", got)
	}
	if got := describe(t, "mkv", audio("A_FLAC", 24)); !strings.Contains(got, "24-bit") {
		t.Errorf("lost the bit depth for flac:\n%s", got)
	}
}

// An unmapped codec is still worth naming, minus the prefix that only says which
// kind of track it is.
func TestMediaMetadataMatroskaUnknownCodec(t *testing.T) {
	data := matroskaFile(ebmlElem(mkvTracks, ebmlElem(mkvTrackEntry,
		ebmlUintElem(mkvTrackType, 1),
		ebmlElem(mkvCodecID, []byte("V_SOMETHING")),
		ebmlElem(mkvVideo, ebmlUintElem(mkvPixelWidth, 16), ebmlUintElem(mkvPixelHeigh, 16)))))

	if got := describe(t, "mkv", data); !strings.Contains(got, "Video: something, 16x16") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataMatroskaDegradesToContainer(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"wrong magic", []byte{0x1a, 0x45, 0xdf, 0xa4, 0x84, 0, 0, 0, 0}},
		{"header only", ebmlElem(ebmlHeader, []byte{0x42, 0x86, 0x81, 0x01})},
		{"truncated segment", matroskaFile(ebmlElem(mkvInfo))[:8]},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(t, "mkv", c.data); got != "Container: mkv" {
				t.Errorf("got:\n%s", got)
			}
		})
	}
}
