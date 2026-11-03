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

	fifo := filepath.Join(dir, "in.fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	go func() {
		// Opening the write end waits for the run to open the read end.
		w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		defer w.Close()
		_, _ = w.Write([]byte("from a fifo\nsecond line\n"))
	}()

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
	})
}
