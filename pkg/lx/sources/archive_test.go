package sources

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/walker"
)

func filePaths(files []InputFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

func TestIsArchivePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"archive.zip", true}, {"lib.jar", true}, {"data.tgz", true}, {"backup.7z", true},
		{"Data.TAR.GZ", true}, {"file.gz", true}, {"dir/sub/data.tar.gz", true},
		{"file.go", false}, {"noextension", false}, {"", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := IsArchivePath(tc.path)
			if got != tc.want {
				t.Errorf("IsArchivePath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// bytesOf returns c's bytes.
func bytesOf(t *testing.T, c *content) string {
	t.Helper()
	data, err := io.ReadAll(c.open())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadEntry_StopsPastLimit(t *testing.T) {
	c, n, err := readEntry(strings.NewReader("0123456789"), -1, 4)
	if err != nil || c != nil || n != 5 {
		t.Errorf("readEntry = %v, %d, %v; want no content, 5", c, n, err)
	}
	c, n, err = readEntry(strings.NewReader("0123"), 2, 4)
	if err != nil || bytesOf(t, c) != "0123" || n != 4 {
		t.Errorf("readEntry = %d, %v; want 0123, 4 despite a wrong hint", n, err)
	}
}

// Whatever size an archive claims, the chunks content is held in add up to
// its size, but for a byte of the last, honest or not: a claim of 1 TB on
// six bytes, a claim of 6 bytes on 100 kB, content of unknown size and
// content just past a chunk. The content reads back whole, at any offset.
func TestReadEntry_HoldsItsSize(t *testing.T) {
	for _, c := range []struct {
		size int
		hint int64
	}{
		{6, 1 << 40}, {6, 1 << 62}, {6, math.MaxInt64}, {6, 6}, {6, -1}, {0, -1}, {0, 0},
		{100 << 10, 6}, {100 << 10, -1}, {firstChunk + 1, firstChunk + 1}, {300 << 10, 300 << 10},
		{3*maxChunk + 7, -1}, {3*maxChunk + 7, 3*maxChunk + 7},
	} {
		want := make([]byte, c.size)
		for i := range want {
			want[i] = byte(i % 251)
		}
		got, n, err := readEntry(bytes.NewReader(want), c.hint, 0)
		if err != nil || n != int64(c.size) || got.size != n {
			t.Fatalf("size %d, hint %d: read %d bytes, %v", c.size, c.hint, n, err)
		}
		held := 0
		for _, chunk := range got.chunks {
			held += cap(chunk)
		}
		if held > c.size+maxSlack {
			t.Errorf("size %d, hint %d: holds %d bytes", c.size, c.hint, held)
		}
		if bytesOf(t, got) != string(want) {
			t.Errorf("size %d, hint %d: content differs", c.size, c.hint)
		}
		if c.size > 10 {
			part := make([]byte, 10)
			if _, err := got.ReadAt(part, int64(c.size-10)); err != nil || string(part) != string(want[c.size-10:]) {
				t.Errorf("size %d, hint %d: ReadAt the tail = %v", c.size, c.hint, err)
			}
		}
	}
}

// An entry whose reader fails, as one cut off mid-stream or ending short of
// the size its zip claims does, is a failure rather than content; one that
// ends cleanly short of its size hint is what arrived.
func TestReadEntry_CutOffFails(t *testing.T) {
	cut := io.MultiReader(strings.NewReader("part"), iotest.ErrReader(io.ErrUnexpectedEOF))
	if _, _, err := readEntry(cut, 100, 0); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cut off: err = %v, want io.ErrUnexpectedEOF", err)
	}
	if c, _, err := readEntry(strings.NewReader("short"), 100, 0); err != nil || bytesOf(t, c) != "short" {
		t.Errorf("short: %v", err)
	}
}

// An entry whose stored bytes fail their checksum, or whose compressed
// stream is corrupt, is a failure rather than content.
func TestExpandArchive_CorruptEntriesFail(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: "bad.txt", Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte("hello world "), 200)); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		data := buf.Bytes()
		// Past the 30-byte local header and the name, in the entry's data.
		start := 30 + len("bad.txt")
		for i := start + 2; i < start+40; i++ {
			data[i] ^= 0x55
		}
		path := filepath.Join(t.TempDir(), "corrupt.zip")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}

		files, failures, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "corrupt.zip", Origin: path, Walker: walker.New(walker.Options{Log: slog.Default()}), Log: slog.Default()})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 || len(failures) != 1 || failures[0].Kind != core.FailedArchiveEntry || failures[0].Display != "corrupt.zip/bad.txt" || failures[0].Path != path+"/bad.txt" {
			t.Errorf("method %d: files %v, failures %+v; want one entry failure", method, filePaths(files), failures)
		}
	}
}

func TestSingleEntryName(t *testing.T) {
	for origin, want := range map[string]string{
		"/tmp/notes.txt.gz":                    "notes.txt",
		"/tmp/.gz":                             ".gz",
		"/tmp/foo.tar.gz":                      "foo.tar",
		"https://example.com/a/log.txt.gz?x=1": "log.txt",
	} {
		if got := singleEntryName(origin); got != want {
			t.Errorf("singleEntryName(%q) = %q, want %q", origin, got, want)
		}
	}
}

// A wrapper is the one directory every entry is below, named by the first.
func TestZipWrapper(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"repo-main/", "repo-main/a.txt", "repo-main/d/b.txt"}, "repo-main"},
		{[]string{"a/x.txt", "b/x.txt", "b/y.txt"}, ""},
		{[]string{"a/x.txt", "TOP.txt"}, ""},
		{[]string{"repo-main/a.txt", "repo-main-2/b.txt"}, ""},
	} {
		var files []*zip.File
		for _, n := range c.names {
			files = append(files, &zip.File{FileHeader: zip.FileHeader{Name: n}})
		}
		if got := zipWrapper(files); got != c.want {
			t.Errorf("zipWrapper(%q) = %q, want %q", c.names, got, c.want)
		}
	}
}

func TestCompareWalkOrder(t *testing.T) {
	paths := []string{"b", "a.txt", "a/z/y", "a/x", "a-b"}
	sort.Slice(paths, func(i, j int) bool { return compareWalkOrder(paths[i], paths[j]) < 0 })
	want := []string{"a/x", "a/z/y", "a-b", "a.txt", "b"}
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("order %v, want %v", paths, want)
	}
}
