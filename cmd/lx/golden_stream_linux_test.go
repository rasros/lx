package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestGoldenStreams renders inputs that are streams rather than files: a FIFO
// with a writer, and a pipe named through /dev/fd as a shell's <(seq 3) names
// it. Both state a size of zero, which says nothing of what they hold.
func TestGoldenStreams(t *testing.T) {
	dir := t.TempDir()

	fifo(t, dir, "in.fifo", "from a fifo\nsecond line\n")
	fifo(t, dir, "over.fifo", "0123456789abcdefghij\n")
	if err := os.Mkdir(filepath.Join(dir, "walk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "walk", "a.txt"), []byte("a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "walk", "nowriter.fifo"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The pipe is written and closed before the run, as seq would have; its
	// read end stays open in this process, which is what /dev/fd names.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if _, err := w.Write([]byte("1\n2\n3\n")); err != nil {
		t.Fatal(err)
	}
	w.Close()
	// A link gives the pipe a name that stays the same from run to run.
	if err := os.Symlink(fmt.Sprintf("/dev/fd/%d", r.Fd()), filepath.Join(dir, "procsub")); err != nil {
		t.Fatal(err)
	}

	runTestGolden(t, dir, []goldenTestCase{
		{name: "975_stream_fifo_with_writer", args: []string{"in.fifo"}},
		{name: "976_stream_process_substitution", args: []string{"procsub"}},
		{name: "977_stream_over_max_size_fails", args: []string{"-m", "10", "over.fifo"}},
		{name: "979_stream_endless_device_fails", args: []string{"/dev/zero"}},
		// A walk skips the FIFO, which has no writer and would hold it up.
		{name: "982_walk_skips_fifo", args: []string{"walk"}},
	})
}

// fifo makes a FIFO in dir whose writer writes body once a reader opens it.
func fifo(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	go func() {
		// Opening the write end waits for the run to open the read end.
		w, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		defer w.Close()
		_, _ = w.Write([]byte(body))
	}()
}
