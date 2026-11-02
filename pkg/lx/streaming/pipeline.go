package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/render"
)

type seqJob struct {
	seqID int
	item  render.PreparedItem
	out   chan<- render.Result
}

var bufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

var readPool = sync.Pool{New: func() interface{} {
	b := make([]byte, 32*1024)
	return &b
}}

type pipelineTotals struct {
	rows   int64
	binary int
	failed int
}

// reorderWindow is how many dispatched items may wait to be written behind
// the one the consumer is currently waiting on. It bounds the memory held by
// rendered buffers when an early item is slow.
func reorderWindow(workers int) int {
	return 2 * workers
}

// executePipeline renders the prepared items on s.concurrency workers and
// writes them to dest in order. The feeder hands each job a one-slot result
// channel and enqueues that channel in a bounded FIFO, so the consumer reads
// results in order and the feeder stalls once the window is full.
func (s *Stream) executePipeline(ctx context.Context, dest *byteCounter, global core.GlobalContext) (pipelineTotals, error) {
	var totals pipelineTotals
	numWorkers := s.concurrency
	jobsCh := make(chan seqJob, numWorkers)
	pending := make(chan chan render.Result, reorderWindow(numWorkers))

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	// On any return, stop the feeder and workers and wait for them to exit.
	defer func() {
		cancel()
		wg.Wait()
	}()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// One processor per worker: it no longer carries per-item state.
			proc := render.NewProcessor(s.engine, global, s.onFileError)
			proc.SetTokenCounter(s.tokenizer.Estimate)

			for j := range jobsCh {
				if ctx.Err() != nil {
					return
				}

				readBufPtr := readPool.Get().(*[]byte)
				readBuf := *readBufPtr
				buf := bufferPool.Get().(*bytes.Buffer)
				buf.Reset()
				if capHint := render.EstimatePreparedBufferSize(j.item); capHint > 0 {
					buf.Grow(capHint)
				}
				localCounter := &byteCounter{w: buf}

				itemStats, err := proc.RenderPrepared(localCounter, j.item, readBuf)
				readPool.Put(readBufPtr)

				// j.out has one slot reserved for this job, so this never blocks.
				j.out <- render.Result{
					Index:        j.seqID,
					Buffer:       buf,
					Stats:        localCounter.count,
					Rows:         itemStats.Rows,
					IsCompact:    itemStats.IsCompact,
					IsBinary:     itemStats.IsBinary,
					IsError:      itemStats.IsError,
					Err:          err,
					SectionIndex: j.item.Section.Index,
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(pending)
		defer close(jobsCh)
		for _, item := range s.prepared {
			out := make(chan render.Result, 1)
			select {
			case pending <- out:
			case <-ctx.Done():
				return
			}
			select {
			case jobsCh <- seqJob{seqID: item.StreamIndex, item: item, out: out}:
			case <-ctx.Done():
				return
			}
		}
	}()

	layout := render.NewLayoutWriter(dest, s.engine, s.sections, s.engine.UseSeparators)

	written := 0
Loop:
	for out := range pending {
		var res render.Result
		select {
		case res = <-out:
		case <-ctx.Done():
			break Loop
		}
		if res.Err != nil {
			return totals, res.Err
		}
		totals.rows += int64(res.Rows)
		if res.IsBinary {
			totals.binary++
		}
		if res.IsError {
			totals.failed++
		}
		if err := layout.WriteItem(res); err != nil {
			return totals, err
		}
		bufferPool.Put(res.Buffer)
		written++
	}

	if err := ctx.Err(); err != nil {
		return totals, err
	}
	if written != len(s.prepared) {
		return totals, fmt.Errorf("pipeline wrote %d of %d items", written, len(s.prepared))
	}
	if err := layout.Close(); err != nil {
		return totals, err
	}
	return totals, nil
}

// byteCounter counts bytes written, plus token estimates when a tokenizer is
// set. Estimating per Write covers separators and wrappers too, not just items.
type byteCounter struct {
	w         io.Writer
	count     int64
	tokens    int64
	tokenizer core.Tokenizer
}

func (b *byteCounter) Write(p []byte) (n int, err error) {
	n, err = b.w.Write(p)
	b.count += int64(n)
	if b.tokenizer != nil && n > 0 {
		b.tokens += b.tokenizer.Estimate(int64(n), p[:n])
	}
	return
}
