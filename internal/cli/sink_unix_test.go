//go:build unix

package cli

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The buffer has no name from the start, so nothing is left behind however
// the process ends.
func TestFileSink_BufferIsUnlinked(t *testing.T) {
	tmp := t.TempDir()
	sink, err := createFileSink(filepath.Join(t.TempDir(), "out.md"), "out.md", tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Abort()
	if names := entries(t, tmp); len(names) != 0 {
		t.Errorf("temp dir holds %v while writing, want nothing", names)
	}
}

// A new file gets the mode os.Create gives it, which the umask limits.
func TestFileSink_NewFileHonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	path := filepath.Join(t.TempDir(), "out.md")
	commit(t, path, "new")

	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 under umask 077", info.Mode().Perm())
	}
}

// The file is written in place, so every hard link sees the new content.
func TestFileSink_KeepsHardLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	other := filepath.Join(dir, "other.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, other); err != nil {
		t.Skip("hard links unsupported:", err)
	}

	commit(t, path, "new")

	if got := readString(t, other); got != "new" {
		t.Errorf("other link = %q, want the new content", got)
	}
}

// What os.Create cannot write fails up front with its error.
func TestFileSink_NotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anything")
	}
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "read-only.md")
	if err := os.WriteFile(readOnly, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := createFileSink(readOnly, "read-only.md", t.TempDir()); err == nil ||
		err.Error() != "open read-only.md: permission denied" {
		t.Errorf("err = %v, want permission denied for a read-only file", err)
	}

	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	if _, err := createFileSink(filepath.Join(locked, "out.md"), "locked/out.md", t.TempDir()); err == nil ||
		err.Error() != "open locked/out.md: permission denied" {
		t.Errorf("err = %v, want permission denied in a read-only directory", err)
	}
}

// A pipe is written to as the output is produced, and stays a pipe, as does
// a device.
func TestFileSink_StreamsToSpecialFiles(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "out.fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	reader := make(chan *os.File)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			t.Error(err)
		}
		reader <- f
	}()
	sink, err := createFileSink(fifo, "out.fifo", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := <-reader
	defer r.Close()
	if _, err := sink.Write([]byte("streamed")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("streamed"))
	if _, err := io.ReadFull(r, got); err != nil || string(got) != "streamed" {
		t.Errorf("pipe read %q, %v before Commit; want the output streamed", got, err)
	}
	if err := sink.Commit(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the pipe was replaced")
	}

	commit(t, os.DevNull, "discarded")
	if info, err := os.Lstat(os.DevNull); err != nil || info.Mode()&os.ModeDevice == 0 {
		t.Errorf("%s is no longer a device", os.DevNull)
	}
}
