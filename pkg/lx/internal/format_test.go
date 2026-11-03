package internal

import (
	"testing"
)

// isoHeader builds an ftyp box with a major brand and compatible brands,
// followed by an empty free box so the header reads as binary.
func isoHeader(major string, compatible ...string) []byte {
	payload := append([]byte(major), zeros(4)...)
	for _, b := range compatible {
		payload = append(payload, b...)
	}
	return append(box("ftyp", payload), box("free")...)
}

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
		{"tif by name only", "scan.TIF", []byte{0, 0, 0, 0}, "tiff"},

		// A strong signature overrides the extension.
		{"png named jpg", "mislabelled.jpg", pngFile(16, 16, 8, 2), "png"},
		{"flac named mp3", "song.mp3", flacFile(44100, 1, 16, 44100), "flac"},
		{"ID3 is strong", "noext", append([]byte("ID3\x04\x00\x00"), zeros(16)...), "mp3"},

		// Text is never media, whatever it is called or however it starts.
		{"lfs pointer named png", "x.png", lfsPointer, ""},
		{"text named bmp", "a.bmp", []byte("BM looks like a bitmap\n"), ""},
		{"text starting with a strong magic", "a.gif", []byte("GIF89a is a format\n"), ""},

		// Weak signatures count only on binary content.
		{"BM text without a name", "notes", []byte("BM25 notes about ranking\n"), ""},
		{"BM binary without a name", "blob", bmpFile(40, 16, 16, 24), "bmp"},
		{"weak magic loses to the name", "a.wav", bmpFile(40, 16, 16, 24), "wav"},
		// ...and only when the parser makes sense of what follows.
		{"BM then zeros", "counts.dat", append([]byte("BM\xd2\x04"), zeros(60)...), ""},
		{"ico magic then a counter", "data.bin",
			[]byte{0, 0, 1, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21}, ""},

		// ISO brands: compatible brands count, the specific brand wins, and
		// within the family the extension decides.
		{"avif under the generic mif1 brand", "photo", isoHeader("mif1", "avif", "mif1", "miaf"), "avif"},
		{"m4a declared in compatible brands", "song", isoHeader("isom", "M4A ", "isom"), "m4a"},
		{"mp4 named m4a keeps the name", "song.m4a", isoHeader("mp42", "mp42", "isom"), "m4a"},
		{"generic iso without a name", "clip", isoHeader("isom", "isom", "mp42"), "mp4"},
		{"iso content named as another family", "song.mp3", isoHeader("M4A ", "M4A "), "m4a"},

		// Not a registered format at all.
		{"source", "main.go", []byte("package main\n"), ""},
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
		"a.tif":  "image/tiff",
		"a.avif": "image/avif",
		"a.svg":  "image/svg+xml",
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

// A name or a weak signature does not prove the bytes; a strong signature or a
// parse that makes sense of them does.
func TestVerify(t *testing.T) {
	cases := []struct {
		name   string
		format string
		header []byte
		want   bool
	}{
		{"png signature", "png", pngFile(16, 16, 8, 2), true},
		{"png name on other bytes", "png", []byte{0, 1, 2, 3}, false},
		{"real bitmap", "bmp", bmpFile(40, 16, 16, 24), true},
		{"BM then zeros", "bmp", append([]byte("BM"), zeros(60)...), false},
		{"real icon", "ico", icoFile([][2]byte{{16, 16}}), true},
		{"icon entry with a bad depth", "ico", []byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 0xf8, 0xf9, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{"mp3 frame", "mp3", append(mp3Header(1, 3, 9, 0, 2), zeros(64)...), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatByName(c.format).Verify(c.header); got != c.want {
				t.Errorf("Verify = %v, want %v", got, c.want)
			}
		})
	}
	if (*Format)(nil).Verify(pngFile(1, 1, 8, 2)) {
		t.Error("a nil format verified")
	}
}

func TestParseBMPRejectsImplausibleHeaders(t *testing.T) {
	text := []byte("BM25 notes about ranking and retrieval, with more text after it\n")
	cases := map[string][]byte{
		"text after the magic": text,
		"unknown header size":  bmpFile(41, 16, 16, 24),
		"huge width":           bmpFile(40, 1<<20, 16, 24),
		"odd bit depth":        bmpFile(40, 16, 16, 7),
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
