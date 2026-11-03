package sources

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/rasros/lx/pkg/lx/core"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/rasros/lx/pkg/lx/walker"
)

func makeTestZip(t *testing.T, entries [][2]string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "archive_test_*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, e := range entries {
		fw, err := w.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func makeTestTarGz(t *testing.T, entries [][2]string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "archive_test_*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		body := []byte(e[1])
		hdr := &tar.Header{Name: e[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

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
		{"archive.zip", true}, {"lib.jar", true}, {"app.war", true}, {"deploy.ear", true},
		{"data.tar", true}, {"data.tar.gz", true}, {"data.tgz", true}, {"data.tar.bz2", true},
		{"data.tbz2", true}, {"data.tar.xz", true}, {"data.txz", true}, {"data.tar.zst", true},
		{"data.tar.br", true}, {"data.tar.lz4", true}, {"backup.rar", true}, {"backup.7z", true},
		{"ARCHIVE.ZIP", true}, {"Archive.Zip", true}, {"Data.TAR.GZ", true}, {"file.gz", true},
		{"file.bz2", true}, {"file.xz", true}, {"file.zst", true}, {"file.br", true},
		{"file.lz4", true}, {"file.sz", true}, {"file.s2", true}, {"file.lz", true},
		{"file.go", false}, {"file.txt", false}, {"noextension", false}, {"", false},
		{"dir/sub/archive.zip", true}, {"dir/sub/data.tar.gz", true}, {"dir/sub/main.go", false},
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

func TestExpandArchive_BasicEntries(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"hello.txt", "Hello!"}, {"nested/world.go", "package nested"}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "archive.zip", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	sort.Strings(paths)
	assertContains(t, paths, "archive.zip/hello.txt")
	assertContains(t, paths, "archive.zip/nested/world.go")
}

func TestExpandArchive_DisplayPathPrefix(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"a.txt", "hi"}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "path/to/archive.zip", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "path/to/archive.zip/a.txt" {
		t.Errorf("Path = %q, want path/to/archive.zip/a.txt", files[0].Path)
	}
}

func TestExpandArchive_ContentReadable(t *testing.T) {
	const content = "package main\n"
	path := makeTestZip(t, [][2]string{{"main.go", content}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "archive.zip", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open(context.Background())
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestExpandArchive_OpenIsIndependent(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"a.txt", "AAA"}, {"b.txt", "BBB"}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	want := map[string]string{"arc/a.txt": "AAA", "arc/b.txt": "BBB"}
	for _, f := range files {
		rc, err := f.Open(context.Background())
		if err != nil {
			t.Fatalf("Open(%q) error: %v", f.Path, err)
		}
		got, _ := io.ReadAll(rc)
		rc.Close()
		if string(got) != want[f.Path] {
			t.Errorf("file %q: content = %q, want %q", f.Path, got, want[f.Path])
		}
	}
}

func TestExpandArchive_HiddenFiltered(t *testing.T) {
	path := makeTestZip(t, [][2]string{{".hidden", "secret"}, {"visible.txt", "visible"}})

	w := walker.New(walker.Options{Overrides: []walker.Override{walker.ExcludeOverride(".*")}})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, "arc/visible.txt")
	assertNotContains(t, paths, "arc/.hidden")
}

func TestExpandArchive_HiddenIncluded(t *testing.T) {
	path := makeTestZip(t, [][2]string{{".hidden", "secret"}, {"visible.txt", "visible"}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, "arc/visible.txt")
	assertContains(t, paths, "arc/.hidden")
}

func TestExpandArchive_IncludeFilter(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"main.go", "package main"}, {"util.go", "package main"}, {"readme.txt", "docs"}})

	w := walker.New(walker.Options{Overrides: []walker.Override{walker.IncludeOverride("*.go")}})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, "arc/main.go")
	assertContains(t, paths, "arc/util.go")
	assertNotContains(t, paths, "arc/readme.txt")
}

func TestExpandArchive_ExcludeViaWalkerRule(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"main.go", "package main"}, {"secret.txt", "do not include"}})

	w := walker.New(walker.Options{Overrides: []walker.Override{walker.ExcludeOverride("secret.txt")}})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, "arc/main.go")
	assertNotContains(t, paths, "arc/secret.txt")
}

func TestExpandArchive_EmptyZip(t *testing.T) {
	path := makeTestZip(t, nil)

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files from empty zip, got %d", len(files))
	}
}

func TestExpandArchive_UnknownExtension(t *testing.T) {
	w := walker.New(walker.Options{IgnoreEnabled: true})

	files, _, err := ExpandArchive(context.Background(), "/any/path/file.go", ArchiveOptions{Display: "file.go", Origin: "/any/path/file.go", Walker: w})
	if err != nil {
		t.Fatalf("expected nil for non-archive path, got: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for non-archive path, got %d", len(files))
	}
}

func TestExpandArchive_BadZipPath(t *testing.T) {
	w := walker.New(walker.Options{IgnoreEnabled: true})

	_, _, err := ExpandArchive(context.Background(), "/nonexistent/path/archive.zip", ArchiveOptions{Display: "arc", Origin: "/nonexistent/path/archive.zip", Walker: w})
	if err == nil {
		t.Fatal("expected error for nonexistent archive, got nil")
	}
}

func TestExpandArchive_TarGz_BasicEntries(t *testing.T) {
	path := makeTestTarGz(t, [][2]string{{"hello.txt", "Hello from tar!"}, {"nested/world.go", "package nested"}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "archive.tar.gz", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	sort.Strings(paths)
	assertContains(t, paths, "archive.tar.gz/hello.txt")
	assertContains(t, paths, "archive.tar.gz/nested/world.go")
}

func TestExpandArchive_TarGz_ContentReadable(t *testing.T) {
	const content = "package main\n"
	path := makeTestTarGz(t, [][2]string{{"main.go", content}})

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc.tar.gz", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open(context.Background())
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func makeTestGz(t *testing.T, name, content string) string {
	t.Helper()
	baseName := name[:len(name)-3]
	f, err := os.CreateTemp(t.TempDir(), "archive_test_*_"+baseName+".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	gw.Name = baseName
	if _, err := gw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestExpandArchive_Gz_SingleEntry(t *testing.T) {
	const content = "hello from gzip\n"
	path := makeTestGz(t, "data.txt.gz", content)

	w := walker.New(walker.Options{})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "data.txt.gz", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open(context.Background())
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestExpandArchive_TarGz_IncludeFilter(t *testing.T) {
	path := makeTestTarGz(t, [][2]string{{"main.go", "package main"}, {"notes.txt", "notes"}})

	w := walker.New(walker.Options{Overrides: []walker.Override{walker.IncludeOverride("*.go")}})

	files, _, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "arc.tar.gz", Origin: path, Walker: w})
	if err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, "arc.tar.gz/main.go")
	assertNotContains(t, paths, "arc.tar.gz/notes.txt")
}

func assertContains(t *testing.T, list []string, item string) {
	t.Helper()
	for _, v := range list {
		if v == item {
			return
		}
	}
	t.Errorf("List missing expected item: %s. Got: %v", item, list)
}

func assertNotContains(t *testing.T, list []string, item string) {
	t.Helper()
	for _, v := range list {
		if v == item {
			t.Errorf("List contains unexpected item: %s", item)
		}
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

// An archive's entries take what they hold in memory, chunks and all: 200
// entries of 100 kB hold little more than their 20 MB.
func TestExpandArchive_HoldsContentSize(t *testing.T) {
	const entries, size = 200, 100 << 10
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for i := range entries {
		w, err := zw.Create(fmt.Sprintf("f%03d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
		w.Write(bytes.Repeat([]byte{byte(i)}, size))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	files, failures, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "a.zip", Origin: path})
	runtime.GC()
	runtime.ReadMemStats(&after)
	if err != nil || len(failures) != 0 || len(files) != entries {
		t.Fatalf("expanded %d entries, failures %v, %v", len(files), failures, err)
	}
	if grew := int64(after.HeapAlloc) - int64(before.HeapAlloc); grew > entries*size*11/10 {
		t.Errorf("%d entries of %d bytes hold %d bytes", entries, size, grew)
	}
	runtime.KeepAlive(files)
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

		files, failures, err := ExpandArchive(context.Background(), path, ArchiveOptions{Display: "corrupt.zip", Origin: path})
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
		"/tmp/NOTES.TXT.GZ":                    "NOTES.TXT",
		"/tmp/data.json.zst":                   "data.json",
		"/tmp/.gz":                             ".gz",
		"/tmp/foo.tar.gz":                      "foo.tar",
		"/tmp/foo.tgz":                         "foo.tar",
		"/tmp/NOTAR.TGZ":                       "NOTAR.TAR",
		"https://example.com/a/log.txt.gz?x=1": "log.txt",
	} {
		if got := singleEntryName(origin); got != want {
			t.Errorf("singleEntryName(%q) = %q, want %q", origin, got, want)
		}
	}
}

// A nil Walker keeps every file, as with ExpandArchive.
func TestExpandRepoArchive_NilWalker(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"repo-main/", ""}, {"repo-main/.hidden", "h"}, {"repo-main/a.txt", "a"}})
	files, _, err := ExpandRepoArchive(context.Background(), path, ArchiveOptions{Display: "r", Origin: path})
	if err != nil || len(files) != 2 {
		t.Errorf("files %v, %v; want both", filePaths(files), err)
	}
}

// A wrapper is the one directory every entry is below, named by the first.
func TestZipWrapper(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"repo-main/", "repo-main/a.txt", "repo-main/d/b.txt"}, "repo-main"},
		{[]string{"repo-main/a.txt", "repo-main/d/"}, "repo-main"},
		{[]string{"a/x.txt", "b/x.txt", "b/y.txt"}, ""},
		{[]string{"a/x.txt", "TOP.txt"}, ""},
		{[]string{"TOP.txt", "a/x.txt"}, ""},
		{[]string{"repo-main/a.txt", "repo-main-2/b.txt"}, ""},
		{[]string{"x.txt"}, ""},
		{nil, ""},
	} {
		if got := zipWrapper(c.names); got != c.want {
			t.Errorf("zipWrapper(%q) = %q, want %q", c.names, got, c.want)
		}
	}
}

func TestCheckoutName(t *testing.T) {
	for name, want := range map[string]bool{
		"a.txt": true, `back\slash.txt`: true, "nul\x00x.txt": false, "d/a.txt": true, "repo-main": true,
		"": false, ".": false, "..": false, "/a": false, "a//b": false,
		"repo-main/.": false, "repo-main/..": false, "repo-main/../up.txt": false, "./a": false,
	} {
		if got := checkoutName(name); got != want {
			t.Errorf("checkoutName(%q) = %v, want %v", name, got, want)
		}
	}
}

// An ignore file stored as a symbolic link is not read as patterns: its
// content is a link target, which a checkout would follow.
func TestExpandRepoArchive_LinkedIgnoreFileUnread(t *testing.T) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, e := range []struct {
		name, data string
		mode       fs.FileMode
	}{
		{"repo-main/.gitignore", "*.txt", fs.ModeSymlink | 0o777},
		{"repo-main/a.txt", "a", 0o644},
	} {
		h := &zip.FileHeader{Name: e.name, Method: zip.Store}
		h.SetMode(e.mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.data))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "repo.zip")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	w := walker.New(walker.Options{IgnoreEnabled: true, RepoRoot: true})
	files, _, err := ExpandRepoArchive(context.Background(), path, ArchiveOptions{Display: "r", Origin: path, Walker: w})
	if err != nil || len(filePaths(files)) != 1 || files[0].Path != "r/a.txt" {
		t.Errorf("files %v, %v; want r/a.txt", filePaths(files), err)
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
