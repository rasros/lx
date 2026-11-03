//go:build unix

package sources

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func newFIFO(t *testing.T) (dir, name string) {
	t.Helper()
	dir = t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "in.fifo"), 0o644); err != nil {
		t.Skip("fifos unsupported:", err)
	}
	return dir, "in.fifo"
}

// A stream that ends at its limit is whole; one that goes on past it fails.
func TestStreamLimit(t *testing.T) {
	for _, tc := range []struct {
		body    string
		wantErr bool
	}{
		{"1234", false},
		{"12345", true},
	} {
		dir, name := newFIFO(t)
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		f := NewInputFile(os.DirFS(dir), name, info)
		go func() {
			if w, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0); err == nil {
				_, _ = w.Write([]byte(tc.body))
				w.Close()
			}
		}()
		rc, err := f.Open(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadStream(rc.(*Stream), 4)
		rc.Close()
		if (err != nil) != tc.wantErr || (err == nil && string(got) != "1234") {
			t.Errorf("%q: read %q, %v; want 1234 or a limit error %v", tc.body, got, err, tc.wantErr)
		}
	}
}
