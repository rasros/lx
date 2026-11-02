//go:build unix

package cli

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

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

// A file with another link is written in place, so every link sees the new
// content.
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

// A writable file in a directory the run cannot write to is written in
// place.
func TestFileSink_ReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to read-only directories")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	commit(t, path, "new")

	if got := readString(t, path); got != "new" {
		t.Errorf("file = %q, want the new content", got)
	}
}

// A pipe receives the output and stays a pipe, as does a device.
func TestFileSink_SpecialFiles(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "out.fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	got := make(chan string)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			got <- err.Error()
			return
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		got <- string(data)
	}()

	commit(t, fifo, "through the pipe")

	if s := <-got; s != "through the pipe" {
		t.Errorf("pipe read %q, want the output", s)
	}
	if info, err := os.Lstat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the pipe was replaced")
	}

	commit(t, os.DevNull, "discarded")
	if info, err := os.Lstat(os.DevNull); err != nil || info.Mode()&os.ModeDevice == 0 {
		t.Errorf("%s is no longer a device", os.DevNull)
	}
}
