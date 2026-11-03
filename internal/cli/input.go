package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// untilDone returns what read returns, or ctx's error once ctx is done. A
// read that ctx abandons is left running: a read of stdin cannot be
// interrupted, and a cancelled run is about to end the process anyway.
func untilDone[T any](ctx context.Context, read func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	ch := make(chan result, 1)
	go func() {
		v, err := read()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// readFilenamesFromStdin reads the paths piped to stdin, one per line or, with
// useNull, NUL-terminated, until ctx is done. It reports false when stdin is a
// terminal; any reader that is not a file counts as a pipe.
func readFilenamesFromStdin(ctx context.Context, stdin io.Reader, useNull bool) ([]string, bool, error) {
	if f, ok := stdin.(*os.File); ok {
		info, err := f.Stat()
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return nil, false, nil
		}
	}
	paths, err := untilDone(ctx, func() ([]string, error) { return scanFilenames(stdin, useNull) })
	return paths, true, err
}

func scanFilenames(stdin io.Reader, useNull bool) ([]string, error) {
	sc := bufio.NewScanner(stdin)
	if useNull {
		sc.Split(scanNullTerminated)
	}

	var paths []string
	for sc.Scan() {
		text := sc.Text()
		if !useNull {
			text = strings.TrimSpace(text)
		}

		if text == "" {
			continue
		}
		paths = append(paths, text)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return paths, nil
}

func scanNullTerminated(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
