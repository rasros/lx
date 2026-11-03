package cli

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// A pipe that stays open without writing must not hold up a cancelled run.
func TestReadFilenamesFromStdin_CancelledWhileIdle(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() {
		_, _, err := readFilenamesFromStdin(ctx, r, false)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reading an idle stdin did not stop when cancelled")
	}
}
