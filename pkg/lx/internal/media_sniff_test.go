package internal

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// sniff runs extraction with no container named, the way a suffixless input
// arrives.
func sniff(t *testing.T, data []byte) (string, bool) {
	t.Helper()
	out, ok := MediaMetadata("", bytes.NewReader(data), int64(len(data)))
	return strings.TrimSpace(string(out)), ok
}

func TestSniffIdentifiesContainers(t *testing.T) {
	cases := []struct {
		want string
		data []byte
	}{
		{"png", pngFile(16, 16, 8, 6)},
		{"jpeg", jpegFile(0xc0, 16, 16, 3)},
		{"gif", gifFile(16, 16, 1, 0)},
		{"bmp", bmpFile(40, 16, 16, 24)},
		{"tiff", tiffFile(binary.LittleEndian, 16, 16)},
		{"tiff", tiffFile(binary.BigEndian, 16, 16)},
		{"ico", icoFile([][2]byte{{16, 16}})},
		{"webp", webpFile("VP8L", append([]byte{0x2f}, binary.LittleEndian.AppendUint32(nil, 15|15<<14)...))},
		{"wav", wavFile(1, 1, 8000, 8, 800)},
		{"flac", flacFile(44100, 1, 16, 44100)},
		{"mp3", append(mp3Header(1, 3, 9, 0, 2), zeros(1024)...)},
		{"mp4", mp4File(mvhd(1000, 1000))},
	}

	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			got, ok := sniff(t, c.data)
			if !ok {
				t.Fatalf("%s was not recognised", c.want)
			}
			if !strings.Contains(got, "Container: "+c.want) {
				t.Errorf("got:\n%s\nwant container %q", got, c.want)
			}
		})
	}
}

// The brand is the only thing separating formats that share the box layout.
func TestSniffReadsISOBrand(t *testing.T) {
	cases := []struct{ brand, want string }{
		{"isom", "mp4"},
		{"mp42", "mp4"},
		{"qt  ", "mov"},
		{"M4A ", "m4a"},
		{"avif", "avif"},
		{"heic", "heic"},
		{"mif1", "heic"},
	}

	for _, c := range cases {
		t.Run(c.brand, func(t *testing.T) {
			data := append(box("ftyp", []byte(c.brand), zeros(4)), box("moov", mvhd(1000, 1000))...)
			got, ok := sniff(t, data)
			if !ok || !strings.Contains(got, "Container: "+c.want) {
				t.Errorf("got:\n%s\nwant container %q", got, c.want)
			}
		})
	}
}

// WebM is Matroska with a narrower profile, and only the DocType says which.
func TestSniffReadsMatroskaDocType(t *testing.T) {
	build := func(docType string) []byte {
		header := ebmlElem(ebmlHeader, ebmlElem(mkvDocType, []byte(docType)))
		return append(header, ebmlElem(mkvSegment,
			ebmlElem(mkvInfo, ebmlFloatElem(mkvDuration, 1000)))...)
	}

	for docType, want := range map[string]string{"webm": "webm", "matroska": "mkv"} {
		t.Run(docType, func(t *testing.T) {
			got, ok := sniff(t, build(docType))
			if !ok || !strings.Contains(got, "Container: "+want) {
				t.Errorf("got:\n%s\nwant container %q", got, want)
			}
		})
	}
}

// A name that disagrees with the bytes loses: the bytes cannot be mistaken.
func TestMediaMetadataPrefersBytesOverAWrongName(t *testing.T) {
	out, ok := MediaMetadata("jpeg", bytes.NewReader(pngFile(64, 48, 8, 2)), int64(len(pngFile(64, 48, 8, 2))))
	if !ok {
		t.Fatal("not recognised")
	}
	got := strings.TrimSpace(string(out))
	if !strings.Contains(got, "Container: png") || !strings.Contains(got, "64x48") {
		t.Errorf("got:\n%s", got)
	}
}

// A named container that will not parse still describes itself, since the name is
// evidence the file was meant to be one.
func TestMediaMetadataKeepsANamedContainerThatWillNotParse(t *testing.T) {
	out, ok := MediaMetadata("mp4", bytes.NewReader([]byte("not really an mp4")), 17)
	if !ok {
		t.Fatal("a named container should still be reported")
	}
	if got := strings.TrimSpace(string(out)); got != "Container: mp4" {
		t.Errorf("got:\n%s", got)
	}
}

// Without a name, an unrecognisable file has to be rejected rather than guessed
// at, so that the binary handling describes it instead.
func TestMediaMetadataRejectsNonMediaWithoutAName(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"plain text", []byte("package main\n\nfunc main() {}\n")},
		{"random bytes", []byte{0x17, 0x03, 0x9a, 0xff, 0x00, 0x42, 0xde, 0xad}},
		{"a lone ftyp with nothing in it", box("ftyp", []byte("isom"), zeros(4))},
		{"riff that is neither wav nor webp", []byte("RIFF\x00\x00\x00\x00AVI ")},
		{"an mp3 sync too deep to trust", append(zeros(64), mp3Header(1, 3, 9, 0, 2)...)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, ok := sniff(t, c.data); ok {
				t.Errorf("recognised %q as media:\n%s", c.name, got)
			}
		})
	}
}

// A reserved field is not a frame header, whatever the sync bits say.
func TestSniffRejectsInvalidMP3Headers(t *testing.T) {
	cases := map[string][]byte{
		"free bitrate":     mp3Header(1, 3, 0, 0, 2),
		"reserved bitrate": mp3Header(1, 3, 15, 0, 2),
		"reserved rate":    mp3Header(1, 3, 9, 3, 2),
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := sniff(t, append(header, zeros(512)...)); ok {
				t.Errorf("accepted %s", name)
			}
		})
	}
}

// An ID3 tag is a strong enough signal on its own.
func TestSniffAcceptsID3Tag(t *testing.T) {
	data := append([]byte("ID3\x04\x00\x00"), []byte{0, 0, 1, 0x7f}...)
	data = append(data, zeros(255)...)
	data = append(data, xingFrame(mp3Header(1, 3, 9, 0, 2), 32, 100)...)

	if got, ok := sniff(t, data); !ok || !strings.Contains(got, "Container: mp3") {
		t.Errorf("got:\n%s ok=%v", got, ok)
	}
}
