package internal

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

// The builders below assemble containers field by field, so a test states which
// bytes produce which line. Real encoder output is covered by the golden
// fixtures instead.

func box(typ string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out[:4], uint32(8+len(body)))
	copy(out[4:], typ)
	return append(out, body...)
}

func be16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func le16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func le32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func zeros(n int) []byte { return make([]byte, n) }

// mvhd version 0: creation, modification, timescale, duration.
func mvhd(timescale, duration uint32) []byte {
	return box("mvhd", zeros(4), zeros(8), be32(timescale), be32(duration))
}

// mdhd shares mvhd's layout.
func mdhd(timescale, duration uint32) []byte {
	return box("mdhd", zeros(4), zeros(8), be32(timescale), be32(duration))
}

func hdlr(handler string) []byte {
	return box("hdlr", zeros(4), zeros(4), []byte(handler), zeros(12))
}

// tkhd version 0 places its 16.16 fixed-point dimensions after a 36-byte matrix.
func tkhd(width, height uint16) []byte {
	return box("tkhd", zeros(4), zeros(20), zeros(8), zeros(8), zeros(36),
		be32(uint32(width)<<16), be32(uint32(height)<<16))
}

// videoSampleEntry stores dimensions where an audio entry stores its rate.
func videoSampleEntry(codec string, width, height uint16) []byte {
	return box("stsd", zeros(4), be32(1),
		box(codec, zeros(6), be16(1), zeros(16), be16(width), be16(height), zeros(50)))
}

func audioSampleEntry(codec string, channels, bitDepth uint16, sampleRate uint32) []byte {
	return box("stsd", zeros(4), be32(1),
		box(codec, zeros(6), be16(1), zeros(8),
			be16(channels), be16(bitDepth), zeros(4), be32(sampleRate<<16)))
}

func stts(sampleCount, delta uint32) []byte {
	return box("stts", zeros(4), be32(1), be32(sampleCount), be32(delta))
}

func trak(parts ...[]byte) []byte {
	return box("trak", parts[0], box("mdia", bytes.Join(parts[1:], nil)))
}

func mp4File(parts ...[]byte) []byte {
	return append(box("ftyp", []byte("isom"), zeros(4)), box("moov", bytes.Join(parts, nil))...)
}

func stbl(parts ...[]byte) []byte {
	return box("minf", box("stbl", bytes.Join(parts, nil)))
}

func describe(t *testing.T, container string, data []byte) string {
	t.Helper()
	out, _ := MediaMetadata(container, bytes.NewReader(data), int64(len(data)))
	return strings.TrimSpace(string(out))
}

func TestMediaMetadataMP4VideoAndAudio(t *testing.T) {
	data := mp4File(
		mvhd(1000, 2500),
		trak(tkhd(1920, 1080), hdlr("vide"), mdhd(1000, 2500),
			stbl(videoSampleEntry("avc1", 1920, 1080), stts(75, 33))),
		trak(tkhd(0, 0), hdlr("soun"), mdhd(48000, 120000),
			stbl(audioSampleEntry("mp4a", 2, 16, 48000), stts(120, 1024))),
	)

	got := describe(t, "mp4", data)
	for _, want := range []string{
		"Container: mp4",
		"Duration: 00:00:02.500",
		"Video: h264, 1920x1080, 30.00 fps",
		"Audio: aac, 48000 Hz, stereo",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// A compressed codec's samplesize is fixed at 16 regardless of the stream, so it
// must not be reported.
func TestMediaMetadataOmitsBitDepthForCompressedAudio(t *testing.T) {
	data := mp4File(mvhd(1000, 1000),
		trak(tkhd(0, 0), hdlr("soun"), mdhd(44100, 44100),
			stbl(audioSampleEntry("mp4a", 2, 16, 44100), stts(43, 1024))))

	if got := describe(t, "mp4", data); strings.Contains(got, "bit") {
		t.Errorf("reported a bit depth for aac:\n%s", got)
	}
}

func TestMediaMetadataKeepsBitDepthForLosslessAudio(t *testing.T) {
	data := mp4File(mvhd(1000, 1000),
		trak(tkhd(0, 0), hdlr("soun"), mdhd(44100, 44100),
			stbl(audioSampleEntry("alac", 2, 24, 44100), stts(43, 1024))))

	if got := describe(t, "mp4", data); !strings.Contains(got, "24-bit") {
		t.Errorf("lost the bit depth for alac:\n%s", got)
	}
}

func TestMediaMetadataMP4Version1Header(t *testing.T) {
	// A v1 mvhd widens its timestamps to 64 bits, moving both fields it carries.
	v1 := box("mvhd", []byte{1, 0, 0, 0}, zeros(16),
		be32(1000), binary.BigEndian.AppendUint64(nil, 90000))
	data := append(box("ftyp", []byte("isom"), zeros(4)), box("moov", v1)...)

	if got := describe(t, "mp4", data); !strings.Contains(got, "Duration: 00:01:30.000") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataMP4LargeBoxSize(t *testing.T) {
	// A size of 1 defers to a 64-bit size after the header.
	inner := mvhd(1000, 1000)
	large := append([]byte{0, 0, 0, 1}, "moov"...)
	large = binary.BigEndian.AppendUint64(large, uint64(16+len(inner)))
	large = append(large, inner...)
	data := append(box("ftyp", []byte("isom"), zeros(4)), large...)

	if got := describe(t, "mp4", data); !strings.Contains(got, "Duration: 00:00:01.000") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataMP4FallsBackToTrackDimensions(t *testing.T) {
	// An entry with no dimensions of its own leaves tkhd as the only source.
	data := mp4File(mvhd(1000, 1000),
		trak(tkhd(640, 480), hdlr("vide"), mdhd(1000, 1000),
			stbl(videoSampleEntry("avc1", 0, 0), stts(25, 40))))

	if got := describe(t, "mp4", data); !strings.Contains(got, "640x480") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataMP4UnknownCodecKeepsFourCC(t *testing.T) {
	data := mp4File(mvhd(1000, 1000),
		trak(tkhd(16, 16), hdlr("vide"), mdhd(1000, 1000),
			stbl(videoSampleEntry("xyzw", 16, 16), stts(1, 1000))))

	if got := describe(t, "mp4", data); !strings.Contains(got, "Video: xyzw") {
		t.Errorf("got:\n%s", got)
	}
}

func wavFile(format, channels uint16, sampleRate uint32, bits uint16, dataLen int) []byte {
	byteRate := sampleRate * uint32(channels) * uint32(bits/8)
	fmtChunk := bytes.Join([][]byte{
		le16(format), le16(channels), le32(sampleRate), le32(byteRate),
		le16(channels * bits / 8), le16(bits),
	}, nil)

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	buf.Write(le32(uint32(36 + dataLen)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	buf.Write(le32(uint32(len(fmtChunk))))
	buf.Write(fmtChunk)
	buf.WriteString("data")
	buf.Write(le32(uint32(dataLen)))
	buf.Write(zeros(dataLen))
	return buf.Bytes()
}

func TestMediaMetadataWAV(t *testing.T) {
	got := describe(t, "wav", wavFile(1, 2, 44100, 16, 44100*2*2))
	want := "Container: wav\nDuration: 00:00:01.000\nBitrate: 1.4 Mbps\nAudio: pcm_s16le, 44100 Hz, stereo, 16-bit"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// An unrecognised chunk before fmt must be stepped over, not parsed.
func TestMediaMetadataWAVSkipsUnknownChunks(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	buf.Write(le32(0))
	buf.WriteString("WAVE")
	buf.WriteString("LIST")
	buf.Write(le32(5)) // odd length, so the next chunk starts after a pad byte
	buf.Write([]byte("INFOx"))
	buf.Write(zeros(1))
	buf.Write(wavFile(1, 1, 8000, 8, 8000)[12:])

	if got := describe(t, "wav", buf.Bytes()); !strings.Contains(got, "8000 Hz, mono, 8-bit") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataWAVCompressedFormat(t *testing.T) {
	if got := describe(t, "wav", wavFile(7, 1, 8000, 8, 8000)); !strings.Contains(got, "pcm_mulaw") {
		t.Errorf("got:\n%s", got)
	}
}

func flacFile(sampleRate uint32, channels, bits uint8, totalSamples uint64) []byte {
	packed := uint64(sampleRate)<<44 |
		uint64(channels-1)<<41 |
		uint64(bits-1)<<36 |
		totalSamples&0xf_ffff_ffff

	var buf bytes.Buffer
	buf.WriteString("fLaC")
	buf.Write([]byte{0x80, 0, 0, 34}) // last block, type 0, 34 bytes
	buf.Write(zeros(10))
	buf.Write(binary.BigEndian.AppendUint64(nil, packed))
	buf.Write(zeros(16)) // md5 of the unencoded audio
	return buf.Bytes()
}

func TestMediaMetadataFLAC(t *testing.T) {
	got := describe(t, "flac", flacFile(96000, 2, 24, 192000))
	for _, want := range []string{"Duration: 00:00:02.000", "Audio: flac, 96000 Hz, stereo, 24-bit"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// A stream of unknown length is legal in FLAC; the rest still describes it.
func TestMediaMetadataFLACWithoutSampleCount(t *testing.T) {
	got := describe(t, "flac", flacFile(44100, 1, 16, 0))
	if strings.Contains(got, "Duration") {
		t.Errorf("invented a duration:\n%s", got)
	}
	if !strings.Contains(got, "Audio: flac, 44100 Hz, mono, 16-bit") {
		t.Errorf("got:\n%s", got)
	}
}

// Nothing here may fail: a media file that cannot be parsed still has to appear
// in the bundle, since the alternative is being dropped as binary.
func TestMediaMetadataDegradesToContainer(t *testing.T) {
	cases := []struct {
		name, container string
		data            []byte
	}{
		{"empty", "mp4", nil},
		{"truncated mp4", "mp4", mp4File(mvhd(1000, 1000))[:20]},
		{"mp4 without moov", "mp4", append(box("ftyp", []byte("isom"), zeros(4)), box("mdat", zeros(64))...)},
		{"not an mp4 at all", "mp4", []byte("just some text, honestly")},
		{"truncated wav", "wav", wavFile(1, 1, 8000, 8, 100)[:14]},
		{"wav without fmt", "wav", []byte("RIFF\x00\x00\x00\x00WAVE")},
		{"truncated flac", "flac", flacFile(44100, 1, 16, 100)[:10]},
		{"unknown container", "mkv", []byte{0x1a, 0x45, 0xdf, 0xa3}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := describe(t, c.container, c.data)
			if got != "Container: "+c.container {
				t.Errorf("got:\n%s", got)
			}
		})
	}
}

// A zero timescale would divide by zero if it reached the arithmetic.
func TestMediaMetadataZeroTimescale(t *testing.T) {
	data := mp4File(mvhd(0, 5000))
	if got := describe(t, "mp4", data); strings.Contains(got, "Duration") {
		t.Errorf("got:\n%s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00:00.000"},
		{200 * time.Millisecond, "00:00:00.200"},
		{90 * time.Second, "00:01:30.000"},
		{3*time.Hour + 4*time.Minute + 5*time.Second, "03:04:05.000"},
	}
	for _, c := range cases {
		if got := formatDuration(c.in); got != c.want {
			t.Errorf("formatDuration(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestFormatBitrateAndChannels(t *testing.T) {
	if got := formatBitrate(999); got != "999 bps" {
		t.Errorf("got %q", got)
	}
	if got := formatBitrate(128_000); got != "128 kbps" {
		t.Errorf("got %q", got)
	}
	if got := formatBitrate(2_400_000); got != "2.4 Mbps" {
		t.Errorf("got %q", got)
	}
	if got := formatChannels(6); got != "6 channels" {
		t.Errorf("got %q", got)
	}
}
