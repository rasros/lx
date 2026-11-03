package goldenfixtures

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// SetupArchiveOriginFixture is SetupArchiveFixture with configs/origin.yaml,
// whose content template shows where a file comes from, and
// configs/datauri.yaml, whose template reads each file with dataURI.
func SetupArchiveOriginFixture(t *testing.T) string {
	t.Helper()
	dir := SetupArchiveFixture(t)
	writeOriginConfigs(t, dir)
	return dir
}

func writeOriginConfigs(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "configs/origin.yaml", "file_content_template: \"{{ .Path }} abs=[{{ .AbsPath }}] archive=[{{ .Archive }}] entry=[{{ .Entry }}]\\n\"\n", 0644)
	writeFile(t, dir, "configs/datauri.yaml", "file_content_template: \"{{ .Path }} {{ dataURI .AbsPath }}\\n\"\n", 0644)
}

// SetupArchiveEdgeFixture writes archives that misbehave:
//   - escape.tar, with entries whose paths leave it through "..";
//   - lie.tar, whose one entry claims 1 TB but holds 6 bytes;
//   - lie.zip, whose a.txt claims nearly 4 GB but holds 6 bytes, beside
//     an honest b.txt;
//   - lie3.zip, whose a.txt, b.txt and c.txt claim 1 MB each, beside an
//     honest d.txt;
//   - s100.txt.gz, a single compressed file;
//   - zip-inside.tar.gz and zip-inside.gz, zip archives, and
//     plain-tar.tar.gz, an uncompressed tar archive, named for other formats;
//
// and the configs of SetupArchiveOriginFixture.
func SetupArchiveEdgeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeOriginConfigs(t, dir)
	createTestTar(filepath.Join(dir, "escape.tar"), [][2]string{
		{"../up.txt", "up\n"},
		{"a/../../up2.txt", "up2\n"},
		{"ok.txt", "ok\n"},
	})
	writeBytes(t, filepath.Join(dir, "lie.tar"), lyingTar(t, 1<<40))
	writeBytes(t, filepath.Join(dir, "lie.zip"), lyingZip(t, 0xfffffff0))
	writeBytes(t, filepath.Join(dir, "lie3.zip"), lyingZipOf(t, 1<<20, 3, "a.txt", "b.txt", "c.txt", "d.txt"))
	createTestGz(filepath.Join(dir, "s100.txt.gz"), "one hundred\n")
	createTestZip(filepath.Join(dir, "zip-inside.tar.gz"), [][2]string{{"z.txt", "a zip\n"}})
	createTestZip(filepath.Join(dir, "zip-inside.gz"), [][2]string{{"z.txt", "a zip\n"}})
	createTestTar(filepath.Join(dir, "plain-tar.tar.gz"), [][2]string{{"t.txt", "a tar\n"}})
	return dir
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// lyingTar returns a tar archive of big.txt, holding "hello\n", whose
// header claims claimed bytes, in the base-256 form large sizes take.
func lyingTar(t *testing.T, claimed uint64) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	if err := tw.WriteHeader(&tar.Header{Name: "big.txt", Mode: 0644, Size: 6, Format: tar.FormatGNU}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	size := data[124:136]
	clear(size)
	size[0] = 0x80
	binary.BigEndian.PutUint64(size[4:], claimed)
	copy(data[148:156], "        ")
	sum := 0
	for _, c := range data[:512] {
		sum += int(c)
	}
	copy(data[148:156], fmt.Sprintf("%06o\x00 ", sum))
	return data
}

// lyingZip returns a zip archive of a.txt and b.txt, stored, each holding
// six bytes, whose central directory claims claimed bytes for a.txt.
func lyingZip(t *testing.T, claimed uint32) []byte {
	return lyingZipOf(t, claimed, 1, "a.txt", "b.txt")
}

// lyingZipOf returns a zip archive of names, stored, each holding six
// bytes, whose central directory claims claimed bytes for the first lying.
func lyingZipOf(t *testing.T, claimed uint32, lying int, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(name[:1] + "ello\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	for i, k := 0, 0; k < lying; k++ {
		j := bytes.Index(data[i:], []byte("PK\x01\x02"))
		binary.LittleEndian.PutUint32(data[i+j+24:], claimed)
		i += j + 4
	}
	return data
}
