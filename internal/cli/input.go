package cli

import (
	"bufio"
	"os"
	"strings"
)

func readFilenamesFromStdin() ([]string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}

	sc := bufio.NewScanner(os.Stdin)
	var paths []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}
