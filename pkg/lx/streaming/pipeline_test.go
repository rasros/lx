package streaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
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
	f.Open = func(ctx context.Context) (io.ReadCloser, error) {
		if started != nil {
			close(started)
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return open(ctx)
	}
	return f
}

// countingFile records how many renders have started.
func countingFile(name, body string, started *atomic.Int64) sources.InputFile {
	f := sources.NewBufferInputFile(name, []byte(body))
	open := f.Open
	f.Open = func(ctx context.Context) (io.ReadCloser, error) {
		started.Add(1)
		return open(ctx)
	}
	return f
}

// stalledFile opens at once but sends prefix and then nothing more until ctx
// is done, when its read fails, as an HTTP body that stalls mid-download does.
func stalledFile(name, prefix string, started chan<- struct{}) sources.InputFile {
	stall := sync.OnceFunc(func() { close(started) })
	return sources.InputFile{
		Path: name,
		Size: -1,
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			return io.NopCloser(io.MultiReader(strings.NewReader(prefix), stalledReader{ctx, stall})), nil
		},
	}
}

type stalledReader struct {
	ctx   context.Context
	stall func()
}

func (r stalledReader) Read([]byte) (int, error) {
	r.stall()
	<-r.ctx.Done()
	return 0, r.ctx.Err()
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
	report, err := stream.Execute(ctx, &buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
	if strings.Contains(buf.String(), testFooter) {
		t.Errorf("output footer written after cancellation:\n%s", buf.String())
	}
	if report.TotalFiles != 0 {
		t.Errorf("report %+v for a cancelled run, want none", report)
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
	_, err := stream.Execute(ctx, &buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context.Canceled", err)
	}
	if strings.Contains(buf.String(), testFooter) {
		t.Errorf("output footer written after cancellation:\n%s", buf.String())
	}
}

// A cancel ends a run promptly even while a file is stalled mid-read, and the
// output holds the items written before it in whole, and nothing of the
// stalled one.
func TestPipeline_CancelInterruptsStalledRead(t *testing.T) {
	base := runtime.NumGoroutine()
	stream := newFooterStream(t, 2)
	started := make(chan struct{})
	stream.AddFile(sources.NewBufferInputFile("first.txt", []byte("first body\n")), wholeFile)
	stream.AddFile(stalledFile("stalled.txt", "partial", started), wholeFile)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var buf strings.Builder
	go func() { _, err := stream.Execute(ctx, &buf); done <- err }()

	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute did not return after cancel while a read was stalled")
	}
	out := buf.String()
	if strings.Contains(out, "partial") || strings.Contains(out, "stalled.txt") || strings.Contains(out, testFooter) {
		t.Errorf("output has the stalled file or the footer:\n%s", out)
	}
	waitGoroutines(t, base)
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
	go func() {
		_, err := stream.Execute(context.Background(), &buf)
		done <- err
	}()

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

	_, err := stream.Execute(context.Background(), &failingWriter{limit: 2048})
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
		if _, err := stream.Execute(context.Background(), &buf); err != nil {
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
