package lx

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"os"
	"sort"
	"testing"
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
		hdr := &tar.Header{
			Name: e[0],
			Mode: 0644,
			Size: int64(len(body)),
		}
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

func streamFiles(s *Stream) []InputFile {
	var files []InputFile
	for _, item := range s.items {
		if f, ok := item.(InputFile); ok {
			files = append(files, f)
		}
	}
	return files
}

// newTestStream returns a minimal Stream usable in archive tests.
func newTestStream(t *testing.T) *Stream {
	t.Helper()
	s, err := NewStream(NewConfig(), RunnerConfig{Head: -1})
	if err != nil {
		t.Fatal(err)
	}
	return s
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
		{"archive.zip", true},
		{"lib.jar", true},
		{"app.war", true},
		{"deploy.ear", true},
		{"data.tar", true},
		{"data.tar.gz", true},
		{"data.tgz", true},
		{"data.tar.bz2", true},
		{"data.tbz2", true},
		{"data.tar.xz", true},
		{"data.txz", true},
		{"data.tar.zst", true},
		{"data.tar.br", true},
		{"data.tar.lz4", true},
		{"backup.rar", true},
		{"backup.7z", true},
		{"ARCHIVE.ZIP", true},
		{"Archive.Zip", true},
		{"Data.TAR.GZ", true},
		{"file.gz", true},
		{"file.bz2", true},
		{"file.xz", true},
		{"file.zst", true},
		{"file.br", true},
		{"file.lz4", true},
		{"file.sz", true},
		{"file.s2", true},
		{"file.lz", true},
		{"file.go", false},
		{"file.txt", false},
		{"noextension", false},
		{"", false},
		{"dir/sub/archive.zip", true},
		{"dir/sub/data.tar.gz", true},
		{"dir/sub/main.go", false},
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
	path := makeTestZip(t, [][2]string{
		{"hello.txt", "Hello!"},
		{"nested/world.go", "package nested"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "archive.zip", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	sort.Strings(paths)
	assertContains(t, paths, "archive.zip/hello.txt")
	assertContains(t, paths, "archive.zip/nested/world.go")
}

func TestExpandArchive_DisplayPathPrefix(t *testing.T) {
	path := makeTestZip(t, [][2]string{{"a.txt", "hi"}})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "path/to/archive.zip", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	files := streamFiles(s)
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

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "archive.zip", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	files := streamFiles(s)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open()
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
	path := makeTestZip(t, [][2]string{
		{"a.txt", "AAA"},
		{"b.txt", "BBB"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	files := streamFiles(s)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	want := map[string]string{"arc/a.txt": "AAA", "arc/b.txt": "BBB"}
	for _, f := range files {
		rc, err := f.Open()
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
	path := makeTestZip(t, [][2]string{
		{".hidden", "secret"},
		{"visible.txt", "visible"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, []string{".*"})
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	assertContains(t, paths, "arc/visible.txt")
	assertNotContains(t, paths, "arc/.hidden")
}

func TestExpandArchive_HiddenIncluded(t *testing.T) {
	path := makeTestZip(t, [][2]string{
		{".hidden", "secret"},
		{"visible.txt", "visible"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	assertContains(t, paths, "arc/visible.txt")
	assertContains(t, paths, "arc/.hidden")
}

func TestExpandArchive_IncludeFilter(t *testing.T) {
	path := makeTestZip(t, [][2]string{
		{"main.go", "package main"},
		{"util.go", "package main"},
		{"readme.txt", "docs"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, []string{"*.go"}, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	assertContains(t, paths, "arc/main.go")
	assertContains(t, paths, "arc/util.go")
	assertNotContains(t, paths, "arc/readme.txt")
}

func TestExpandArchive_ExcludeViaWalkerRule(t *testing.T) {
	path := makeTestZip(t, [][2]string{
		{"main.go", "package main"},
		{"secret.txt", "do not include"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, []string{"secret.txt"})
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	assertContains(t, paths, "arc/main.go")
	assertNotContains(t, paths, "arc/secret.txt")
}

func TestExpandArchive_EmptyZip(t *testing.T) {
	path := makeTestZip(t, nil)

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	if len(streamFiles(s)) != 0 {
		t.Errorf("expected 0 files from empty zip, got %d", len(streamFiles(s)))
	}
}

func TestExpandArchive_UnknownExtension(t *testing.T) {
	s := newTestStream(t)
	w := NewWalker(nil, nil)

	if err := ExpandArchive(context.Background(), "/any/path/file.go", "file.go", w, nil, "", s); err != nil {
		t.Fatalf("expected nil for non-archive path, got: %v", err)
	}
	if len(streamFiles(s)) != 0 {
		t.Errorf("expected 0 files for non-archive path, got %d", len(streamFiles(s)))
	}
}

func TestExpandArchive_BadZipPath(t *testing.T) {
	s := newTestStream(t)
	w := NewWalker(nil, nil)

	err := ExpandArchive(context.Background(), "/nonexistent/path/archive.zip", "arc", w, nil, "", s)
	if err == nil {
		t.Fatal("expected error for nonexistent archive, got nil")
	}
}

func TestExpandArchive_TarGz_BasicEntries(t *testing.T) {
	path := makeTestTarGz(t, [][2]string{
		{"hello.txt", "Hello from tar!"},
		{"nested/world.go", "package nested"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "archive.tar.gz", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	sort.Strings(paths)
	assertContains(t, paths, "archive.tar.gz/hello.txt")
	assertContains(t, paths, "archive.tar.gz/nested/world.go")
}

func TestExpandArchive_TarGz_ContentReadable(t *testing.T) {
	const content = "package main\n"
	path := makeTestTarGz(t, [][2]string{{"main.go", content}})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc.tar.gz", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	files := streamFiles(s)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open()
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

// makeTestGz creates a plain gzip-compressed file (not tar) from a single payload.
// The decompressed name the library exposes is derived from the filename (strip .gz).
func makeTestGz(t *testing.T, name, content string) string {
	t.Helper()
	// Use * before the extension so os.CreateTemp inserts the random part there,
	// keeping the .gz suffix so IsArchivePath recognises the file.
	baseName := name[:len(name)-3] // strip .gz
	f, err := os.CreateTemp(t.TempDir(), "archive_test_*_"+baseName+".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	gw.Name = baseName // embedded name inside the gz stream
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

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "data.txt.gz", w, nil, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	files := streamFiles(s)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	rc, err := files[0].Open()
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
	path := makeTestTarGz(t, [][2]string{
		{"main.go", "package main"},
		{"notes.txt", "notes"},
	})

	s := newTestStream(t)
	w := NewWalker(nil, nil)
	w.IgnoreEnabled = false

	if err := ExpandArchive(context.Background(), path, "arc.tar.gz", w, []string{"*.go"}, "", s); err != nil {
		t.Fatalf("ExpandArchive error: %v", err)
	}

	paths := filePaths(streamFiles(s))
	assertContains(t, paths, "arc.tar.gz/main.go")
	assertNotContains(t, paths, "arc.tar.gz/notes.txt")
}
