package streaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

const testFooter = "<<END-OF-OUTPUT>>"

// wholeFile renders every line of a file.
var wholeFile = core.RenderOptions{Head: -1, Tail: -1}

func newFooterStream(t *testing.T, workers int) *Stream {
	t.Helper()
	cfg := core.NewConfig()
	cfg.OutputFooterTemplate = testFooter
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	return stream.WithConcurrency(workers)
}

// gatedFile is a file whose Open blocks until gate is closed or ctx is done.
func gatedFile(name, body string, gate <-chan struct{}, started chan<- struct{}) sources.InputFile {
	f := sources.NewBufferInputFile(name, []byte(body))
	open := f.Open
	f.Open = func() (io.ReadCloser, error) {
		if started != nil {
			close(started)
		}
		<-gate
		return open()
	}
	return f
}

// countingFile records how many renders have started.
func countingFile(name, body string, started *atomic.Int64) sources.InputFile {
	f := sources.NewBufferInputFile(name, []byte(body))
	open := f.Open
	f.Open = func() (io.ReadCloser, error) {
		started.Add(1)
		return open()
	}
	return f
}

// waitGoroutines waits for the goroutine count to settle at or below base.
func waitGoroutines(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= base {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutines leaked: %d running, baseline %d", n, base)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPipeline_CancelledContextFailsWithoutFooter(t *testing.T) {
	stream := newFooterStream(t, 4)
	for i := 0; i < 200; i++ {
		stream.AddFile(sources.NewBufferInputFile(fmt.Sprintf("f%d.txt", i), []byte("data")), wholeFile)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf strings.Builder
	err := stream.Execute(ctx, &buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
	if strings.Contains(buf.String(), testFooter) {
		t.Errorf("output footer written after cancellation:\n%s", buf.String())
	}
	if stream.finalStats != nil {
		t.Error("final stats recorded for a cancelled run")
	}
}

func TestPipeline_CancelMidRunFailsWithoutFooter(t *testing.T) {
	stream := newFooterStream(t, 2)
	gate := make(chan struct{})
	started := make(chan struct{})
	stream.AddFile(gatedFile("slow.txt", "slow", gate, started), wholeFile)
	for i := 0; i < 20; i++ {
		stream.AddFile(sources.NewBufferInputFile(fmt.Sprintf("f%d.txt", i), []byte("data")), wholeFile)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
		close(gate)
	}()

	var buf strings.Builder
	err := stream.Execute(ctx, &buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
	if strings.Contains(buf.String(), testFooter) {
		t.Errorf("output footer written after cancellation:\n%s", buf.String())
	}
}

func TestPipeline_SlowHeadBoundsBufferedResults(t *testing.T) {
	const workers = 4
	const fastItems = 200
	window := reorderWindow(workers)

	stream := newFooterStream(t, workers)
	gate := make(chan struct{})
	headStarted := make(chan struct{})
	var started atomic.Int64
	stream.AddFile(gatedFile("slow.txt", "slow", gate, headStarted), wholeFile)
	for i := 0; i < fastItems; i++ {
		stream.AddFile(countingFile(fmt.Sprintf("f%d.txt", i), "data", &started), wholeFile)
	}

	done := make(chan error, 1)
	var buf strings.Builder
	go func() { done <- stream.Execute(context.Background(), &buf) }()

	<-headStarted
	// Give the fast items every chance to run ahead of the blocked head.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if n := started.Load(); n > int64(window) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	// Dispatched jobs behind the head are bounded by the window: the head job
	// plus at most `window` queued result slots.
	if n := started.Load(); n > int64(window) {
		t.Errorf("%d later items rendered while the head was blocked, want at most %d", n, window)
	}
	close(gate)

	if err := <-done; err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if n := started.Load(); n != fastItems {
		t.Errorf("rendered %d fast items, want %d", n, fastItems)
	}
	if !strings.HasSuffix(strings.TrimSpace(buf.String()), testFooter) {
		t.Error("output footer missing after successful run")
	}
}

// failingWriter fails every write after the first limit bytes.
type failingWriter struct {
	limit int
	n     int
}

var errDestFull = errors.New("destination full")

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.n+len(p) > w.limit {
		return 0, errDestFull
	}
	w.n += len(p)
	return len(p), nil
}

func TestPipeline_WriteErrorReturnedWithoutLeaks(t *testing.T) {
	base := runtime.NumGoroutine()

	stream := newFooterStream(t, 4)
	for i := 0; i < 500; i++ {
		stream.AddFile(sources.NewBufferInputFile(fmt.Sprintf("f%d.txt", i), bytes.Repeat([]byte("x"), 256)), wholeFile)
	}

	err := stream.Execute(context.Background(), &failingWriter{limit: 2048})
	if !errors.Is(err, errDestFull) {
		t.Fatalf("Execute error = %v, want %v", err, errDestFull)
	}
	waitGoroutines(t, base)
}

func TestPipeline_PreservesOrderWithConcurrency(t *testing.T) {
	const n = 300
	build := func(workers int) string {
		stream := newFooterStream(t, workers)
		for i := 0; i < n; i++ {
			if i%50 == 0 {
				stream.AddSection(fmt.Sprintf("Section %d", i/50))
			}
			stream.AddFile(sources.NewBufferInputFile(fmt.Sprintf("f%03d.txt", i), []byte(fmt.Sprintf("item-%03d", i))), wholeFile)
		}
		var buf strings.Builder
		if err := stream.Execute(context.Background(), &buf); err != nil {
			t.Fatalf("Execute(workers=%d): %v", workers, err)
		}
		return buf.String()
	}

	serial := build(1)
	last := -1
	for i := 0; i < n; i++ {
		idx := strings.Index(serial, fmt.Sprintf("item-%03d", i))
		if idx < 0 || idx < last {
			t.Fatalf("item-%03d out of order in serial output", i)
		}
		last = idx
	}
	for _, workers := range []int{2, 8} {
		if got := build(workers); got != serial {
			t.Errorf("output with %d workers differs from serial output", workers)
		}
	}
}
