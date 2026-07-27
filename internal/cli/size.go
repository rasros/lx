package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rasros/lx/pkg/lx"
)

// Multipliers are decimal to match the humanize template function, which
// renders sizes as kB/MB/GB.
var sizeUnits = []struct {
	suffix string
	mult   int64
}{
	{"gb", 1000 * 1000 * 1000},
	{"mb", 1000 * 1000},
	{"kb", 1000},
	{"g", 1000 * 1000 * 1000},
	{"m", 1000 * 1000},
	{"k", 1000},
	{"b", 1},
}

// parseSizeLimit parses a byte count with an optional unit suffix, such as
// "4096", "512k", or "2MB". Suffixes are case-insensitive.
func parseSizeLimit(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	lower := strings.ToLower(s)
	mult := int64(1)
	for _, u := range sizeUnits {
		if len(lower) > len(u.suffix) && strings.HasSuffix(lower, u.suffix) {
			lower = strings.TrimSuffix(lower, u.suffix)
			mult = u.mult
			break
		}
	}

	n, err := strconv.ParseInt(lower, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q, expected a byte count like 512k or 2M", raw)
	}
	if n < 0 {
		return 0, fmt.Errorf("size cannot be negative: %q", raw)
	}
	if n == 0 {
		return 0, fmt.Errorf("size must be greater than zero")
	}
	if mult > 1 && n > (1<<62)/mult {
		return 0, fmt.Errorf("size out of range: %q", raw)
	}

	return n * mult, nil
}

// overMaxSize matches the guard streaming applies in AddFile, so a file dropped
// from the bundle is dropped from the tree too. A negative size means unknown,
// as for remote inputs, and is never over the limit.
func overMaxSize(cfg lx.RunnerConfig, size int64) bool {
	return cfg.MaxSize > 0 && size > cfg.MaxSize
}
