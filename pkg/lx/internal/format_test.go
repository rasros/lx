package internal

import (
	"encoding/binary"
	"testing"
)

func formatName(f *Format) string {
	if f == nil {
		return ""
	}
	return f.Name
}

func TestClassify(t *testing.T) {
	lfsPointer := []byte("version https://git-lfs.github.com/spec/v1\n" +
		"oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 12345\n")

	cases := []struct {
		name   string
		path   string
		header []byte
		want   string
	}{
		// The extension decides when the content has nothing to say against it.
		{"empty content trusts the name", "x.png", nil, "png"},
		{"binary without a signature trusts the name", "clip.mp4", []byte{0, 1, 2, 3, 0xff}, "mp4"},
		{"tif", "x.tif", tiffFile(binary.LittleEndian, 16, 16), "tiff"},
		{"tiff", "x.tiff", tiffFile(binary.BigEndian, 16, 16), "tiff"},
		{"tif by name only", "scan.TIF", []byte{0, 0, 0, 0}, "tiff"},
		{"heic by name", "photo.heic", []byte{0, 0, 0, 0}, "heic"},

		// A strong signature overrides the extension.
		{"png named jpg", "mislabelled.jpg", pngFile(16, 16, 8, 2), "png"},
		{"png named txt", "image.txt", pngFile(16, 16, 8, 2), "png"},
		{"png without a name", "download", pngFile(16, 16, 8, 2), "png"},
		{"flac named mp3", "song.mp3", flacFile(44100, 1, 16, 44100), "flac"},
		{"ID3 is strong", "noext", append([]byte("ID3\x04\x00\x00"), zeros(16)...), "mp3"},

		// Text is never media, whatever it is called or however it starts.
		{"lfs pointer named png", "x.png", lfsPointer, ""},
		{"lfs pointer named mp4", "clip.mp4", lfsPointer, ""},
		{"text named bmp", "a.bmp", []byte("BM looks like a bitmap\n"), ""},
		{"text starting with a strong magic", "a.gif", []byte("GIF89a is a format\n"), ""},
		{"text starting with ID3", "tags", []byte("ID3 tags explained\n"), ""},

		// Weak signatures count only on binary content.
		{"BM text without a name", "notes", []byte("BM25 notes about ranking\n"), ""},
		{"BM binary without a name", "blob", bmpFile(40, 16, 16, 24), "bmp"},
		{"ico binary without a name", "blob", icoFile([][2]byte{{16, 16}}), "ico"},
		{"mp3 sync binary without a name", "blob", append(mp3Header(1, 3, 9, 0, 2), zeros(64)...), "mp3"},
		{"weak magic loses to the name", "a.wav", bmpFile(40, 16, 16, 24), "wav"},

		// Not a registered format at all.
		{"source", "main.go", []byte("package main\n"), ""},
		{"unknown binary", "blob.dat", []byte{0x00, 0x13, 0x37}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatName(Classify(c.path, c.header)); got != c.want {
				t.Errorf("Classify(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

// Every extension resolves to exactly one format, and every format that can
// be parsed can be found by its name.
func TestRegistryIsConsistent(t *testing.T) {
	seen := map[string]string{}
	for _, f := range formats {
		if f.MIME == "" {
			t.Errorf("%s has no MIME type", f.Name)
		}
		if FormatByName(f.Name) != f {
			t.Errorf("%s is not found by its name", f.Name)
		}
		if f.IsMedia() && f.parse == nil {
			t.Errorf("%s is media but has no parser", f.Name)
		}
		for _, ext := range f.Exts {
			if other, ok := seen[ext]; ok {
				t.Errorf("%s is claimed by both %s and %s", ext, other, f.Name)
			}
			seen[ext] = f.Name
		}
	}
}

func TestIsImageAlternativeExtensions(t *testing.T) {
	for _, p := range []string{"a.tif", "a.tiff", "a.heic", "a.avif", "a.bmp", "a.ico"} {
		if !IsImage(p) {
			t.Errorf("IsImage(%q) = false", p)
		}
	}
	for _, p := range []string{"a.svg", "a.mp4", "a.mp3"} {
		if IsImage(p) {
			t.Errorf("IsImage(%q) = true", p)
		}
	}
}

// The registry answers before the host's MIME table, so these hold on hosts
// with no system table at all.
func TestMIMETypeFallbacks(t *testing.T) {
	cases := map[string]string{
		"a.bmp":  "image/bmp",
		"a.ico":  "image/x-icon",
		"a.tif":  "image/tiff",
		"a.tiff": "image/tiff",
		"a.avif": "image/avif",
		"a.heic": "image/heic",
		"a.svg":  "image/svg+xml",
		"a.webp": "image/webp",
	}
	for p, want := range cases {
		if got := MIMEType(p); got != want {
			t.Errorf("MIMEType(%q) = %q, want %q", p, got, want)
		}
	}
}

// The bytes decide the type of an embedded image where they can.
func TestDataURIUsesContentType(t *testing.T) {
	got := DataURI("mislabelled.jpg", pngFile(1, 1, 8, 2))
	want := "data:image/png;base64,"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("DataURI = %q, want prefix %q", got, want)
	}
}

func TestParseBMPRejectsImplausibleHeaders(t *testing.T) {
	text := []byte("BM25 notes about ranking and retrieval, with more text after it\n")
	cases := map[string][]byte{
		"text after the magic": text,
		"unknown header size":  bmpFile(41, 16, 16, 24),
		"zero width":           bmpFile(40, 0, 16, 24),
		"huge width":           bmpFile(40, 1<<20, 16, 24),
		"huge height":          bmpFile(40, 16, -(1 << 20), 24),
		"odd bit depth":        bmpFile(40, 16, 16, 7),
		"core header bad bits": bmpFile(12, 16, 16, 3),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if info := parseBMP(headerReader(data)); info != nil {
				t.Errorf("accepted %s: %+v", name, *info.Image)
			}
		})
	}

	for _, bits := range []uint16{1, 4, 8, 16, 24, 32} {
		if parseBMP(headerReader(bmpFile(40, 16, 16, bits))) == nil {
			t.Errorf("rejected a %d-bit bitmap", bits)
		}
	}
}
