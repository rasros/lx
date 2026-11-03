package internal

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestContextReaderAt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := ContextReaderAt(ctx, strings.NewReader("hello world"))

	buf := make([]byte, 5)
	if n, err := r.ReadAt(buf, 6); n != 5 || err != nil || string(buf) != "world" {
		t.Fatalf("live ReadAt = %d, %v, %q; want 5, nil, world", n, err, buf)
	}
	// Errors of the underlying reader pass through while ctx is live.
	if _, err := r.ReadAt(buf, 20); err != io.EOF {
		t.Fatalf("ReadAt past the end = %v, want io.EOF", err)
	}

	cancel()
	if n, err := r.ReadAt(buf, 0); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadAt after cancel = %d, %v; want 0, context.Canceled", n, err)
	}
}
