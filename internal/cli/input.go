package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

func readFilenamesFromStdin(useNull bool) ([]string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}

	sc := bufio.NewScanner(os.Stdin)
	if useNull {
		sc.Split(scanNullTerminated)
	}

	var paths []string
	for sc.Scan() {
		text := sc.Text()
		// Only trim space for standard newline-separated input.
		// For -0, filenames preserve whitespace (though we still skip empty).
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
