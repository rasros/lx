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
