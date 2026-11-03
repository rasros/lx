//go:build unix

package sources

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func newFIFO(t *testing.T) (dir, name string) {
	t.Helper()
	dir = t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "in.fifo"), 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	return dir, "in.fifo"
}

// waitCancelled runs op, cancels it after a moment and fails unless it then
// returns ctx's error promptly.
func waitCancelled(t *testing.T, op func(ctx context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(20*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() { done <- op(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not stop when cancelled")
	}
}

// Opening a FIFO waits for a writer, which may never come.
func TestOpenFIFOWithoutWriterStopsWhenCancelled(t *testing.T) {
	dir, name := newFIFO(t)
	f, err := NewInputFileFromPath(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	waitCancelled(t, func(ctx context.Context) error {
		rc, err := f.Open(ctx)
		if err == nil {
			rc.Close()
		}
		return err
	})
	// Let the abandoned open finish, so the test leaves nothing behind.
	if w, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0); err == nil {
		w.Close()
	}
}

// A FIFO whose writer stays open without writing blocks its reads.
func TestReadIdleFIFOStopsWhenCancelled(t *testing.T) {
	dir, name := newFIFO(t)
	f, err := NewInputFileFromPath(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	// The writer opens once the reader does, and then writes nothing.
	w2 := make(chan *os.File, 1)
	go func() {
		w, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0)
		if err != nil {
			close(w2)
			return
		}
		w2 <- w
	}()
	waitCancelled(t, func(ctx context.Context) error {
		rc, err := f.Open(ctx)
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.ReadAll(rc)
		return err
	})
	if w := <-w2; w != nil {
		w.Close()
	}
}
