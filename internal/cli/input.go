package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

func readFilenamesFromStdin(useNull bool) ([]string, bool, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, false, err
	}

	// If CharDevice is set, it's a terminal (interactive).
	// If it is NOT set, it's a pipe or file redirection.
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, false, nil
	}

	sc := bufio.NewScanner(os.Stdin)
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
		return nil, true, fmt.Errorf("read stdin: %w", err)
	}

	return paths, true, nil
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
