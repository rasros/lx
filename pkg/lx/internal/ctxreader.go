package internal

import (
	"context"
	"io"
)

// ContextReaderAt returns r reading only while ctx is live: once ctx is done,
// every read fails with ctx's error. A parser that reads as it goes, as the
// document libraries do, then stops at its next read.
func ContextReaderAt(ctx context.Context, r io.ReaderAt) io.ReaderAt {
	return ctxReaderAt{ctx: ctx, r: r}
}

type ctxReaderAt struct {
	ctx context.Context
	r   io.ReaderAt
}

func (c ctxReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.ReadAt(p, off)
}
