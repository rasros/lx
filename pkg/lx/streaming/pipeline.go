package streaming

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/render"
)

type seqJob struct {
	seqID int
	item  render.PreparedItem
}

var bufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

var readPool = sync.Pool{New: func() interface{} {
	b := make([]byte, 32*1024)
	return &b
}}

func (s *Stream) executePipeline(ctx context.Context, dest *byteCounter, global core.GlobalContext) error {
	numWorkers := s.concurrency
	jobsCh := make(chan seqJob, numWorkers)
	resultsCh := make(chan render.Result, numWorkers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsCh {
				if ctx.Err() != nil {
					return
				}

				proc := render.NewProcessor(s.engine, global, s.onFileError, s.format, s.extractDocuments)
				proc.SetTokenCounter(s.tokenizer.Estimate)

				readBufPtr := readPool.Get().(*[]byte)
				readBuf := *readBufPtr
				buf := bufferPool.Get().(*bytes.Buffer)
				buf.Reset()
				localCounter := &byteCounter{w: buf}

				err := proc.RenderPrepared(localCounter, j.item, readBuf)
				readPool.Put(readBufPtr)

				select {
				case resultsCh <- render.Result{
					Index:        j.seqID,
					Buffer:       buf,
					Stats:        localCounter.count,
					IsCompact:    proc.LastWasCompact(),
					Err:          err,
					SectionIndex: j.item.Section.Index,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
	Loop:
		for _, item := range s.prepared {
			select {
			case jobsCh <- seqJob{seqID: item.StreamIndex, item: item}:
			case <-ctx.Done():
				break Loop
			}
		}
		close(jobsCh)
		wg.Wait()
		close(resultsCh)
	}()

	useSeparators := s.format != "html"
	layout := render.NewLayoutWriter(dest, s.engine, s.sections, useSeparators)
	defer layout.Close()

	nextSeq := 0
	buffer := make(map[int]render.Result)

	for res := range resultsCh {
		if res.Err != nil {
			return res.Err
		}
		buffer[res.Index] = res

		for {
			next, ok := buffer[nextSeq]
			if !ok {
				break
			}
			if err := layout.WriteItem(next); err != nil {
				return err
			}
			bufferPool.Put(next.Buffer)
			delete(buffer, nextSeq)
			nextSeq++
		}
	}
	return nil
}

type byteCounter struct {
	w     io.Writer
	count int64
}

func (b *byteCounter) Write(p []byte) (n int, err error) {
	n, err = b.w.Write(p)
	b.count += int64(n)
	return
}
