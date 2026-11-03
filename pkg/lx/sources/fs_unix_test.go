//go:build unix

package sources

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// waitGoroutines waits for the goroutine count to settle at or below n.
func waitGoroutines(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > n {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines running, want at most %d", runtime.NumGoroutine(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Opening a FIFO waits for a writer, which may never come. Each abandoned
// open leaves only the goroutine blocked in it, which ends once the open
// returns.
func TestOpenFIFOWithoutWriterStopsWhenCancelled(t *testing.T) {
	base := runtime.NumGoroutine()
	dir, name := newFIFO(t)
	f, err := NewInputFileFromPath(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	const opens = 3
	for i := 0; i < opens; i++ {
		waitCancelled(t, func(ctx context.Context) error {
			rc, err := f.Open(ctx)
			if err == nil {
				rc.Close()
			}
			return err
		})
	}
	waitGoroutines(t, base+opens)

	// A writer releases every open waiting on the FIFO.
	if w, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0); err == nil {
		w.Close()
	}
	waitGoroutines(t, base)
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
	// Open reads the stream whole, so it is Open that waits on the writer.
	waitCancelled(t, func(ctx context.Context) error {
		rc, err := f.Open(ctx)
		if err == nil {
			rc.Close()
		}
		return err
	})
	if w := <-w2; w != nil {
		w.Close()
	}
}

// A FIFO reads as what its writer wrote, though it states a size of zero.
func TestOpenFIFOReadsWhatWasWritten(t *testing.T) {
	dir, name := newFIFO(t)
	f, err := NewInputFileFromPath(os.DirFS(dir), name)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		if w, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0); err == nil {
			_, _ = w.Write([]byte("streamed\n"))
			w.Close()
		}
	}()
	rc, err := f.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if got, err := io.ReadAll(rc); err != nil || string(got) != "streamed\n" {
		t.Fatalf("read %q, %v; want %q", got, err, "streamed\n")
	}
}

// A device that never ends is read only up to maxStreamSize.
func TestOpenEndlessDeviceStops(t *testing.T) {
	f, err := NewInputFileFromPath(os.DirFS("/dev"), "zero")
	if err != nil {
		t.Skip("no /dev/zero:", err)
	}
	rc, err := f.Open(context.Background())
	if err == nil {
		rc.Close()
		t.Fatal("Open of /dev/zero succeeded, want it stopped at the size cap")
	}
}
