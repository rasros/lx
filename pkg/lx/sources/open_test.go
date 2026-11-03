package sources

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type closeRecorder struct{ closed chan struct{} }

func (c closeRecorder) Close() error { close(c.closed); return nil }

// An open a cancel abandoned closes what it opens once it returns, and a
// WaitGroup counting it is done only then.
func TestOpenUntilDoneAbandoned(t *testing.T) {
	var wg sync.WaitGroup
	release := make(chan struct{})
	rec := closeRecorder{closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	_, err := openUntilDone(ctx, &wg, func() (closeRecorder, error) {
		<-release
		return rec, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	waited := make(chan struct{})
	go func() { wg.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("WaitGroup done while the abandoned open still runs")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-rec.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("abandoned open did not close what it opened")
	}
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitGroup not done after the abandoned open ended")
	}
}

// A ctx done before the open starts it not at all.
func TestOpenUntilDoneDoneBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := openUntilDone(ctx, nil, func() (closeRecorder, error) {
		t.Error("open ran with ctx already done")
		return closeRecorder{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFormatSize(t *testing.T) {
	for n, want := range map[int64]string{10: "10 B", 1000: "1 kB", 1500: "1.5 kB", DefaultStreamLimit: "64 MB"} {
		if got := formatSize(n); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", n, got, want)
		}
	}
}
