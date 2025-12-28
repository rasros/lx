package lx

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	Head             int
	Tail             int
	PrefixDelimiter  string
	PostfixDelimiter string
	LineNumbers      bool
	ShowTokens       bool
}

// platform-specific newline placeholder replacement
var nl = func() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}()

// NewRunner constructs a Runner with default delimiters if none are provided.
func NewRunner(head, tail int, prefix, postfix string, lineNumbers, ShowTokens bool) Runner {
	if prefix == "" {
		prefix = "{filename} ({row_count} rows){n}---{n}```{language}{n}"
	}
	if postfix == "" {
		postfix = "```{n}{n}"
	}
	return Runner{
		Head:             head,
		Tail:             tail,
		PrefixDelimiter:  prefix,
		PostfixDelimiter: postfix,
		LineNumbers:      lineNumbers,
		ShowTokens:       ShowTokens,
	}
}

func (r Runner) buildPrefix(path string, totalRows int, byteSize int64, lastMod, lang string) string {
	prefix := r.PrefixDelimiter
	prefix = strings.ReplaceAll(prefix, "{filename}", path)
	prefix = strings.ReplaceAll(prefix, "{row_count}", strconv.Itoa(totalRows))
	prefix = strings.ReplaceAll(prefix, "{byte_size}", strconv.FormatInt(byteSize, 10))
	prefix = strings.ReplaceAll(prefix, "{last_modified}", lastMod)
	prefix = strings.ReplaceAll(prefix, "{language}", lang)
	prefix = strings.ReplaceAll(prefix, "{n}", nl)
	return prefix
}

func (r Runner) buildPostfix() string {
	return strings.ReplaceAll(r.PostfixDelimiter, "{n}", nl)
}

func (r Runner) runFile(path string, out io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}

	view, totalRows := prepareView(data, r.Head, r.Tail)

	byteSize := info.Size()
	lastMod := info.ModTime().Format(time.RFC3339)
	lang := DetectLanguage(path, data)

	prefix := r.buildPrefix(path, totalRows, byteSize, lastMod, lang)

	if _, err := out.Write([]byte(prefix)); err != nil {
		return fmt.Errorf("write prefix: %w", err)
	}

	var toWrite []byte
	if r.LineNumbers {
		toWrite = addLineNumbers(view, totalRows, r.Head, r.Tail)
	} else {
		toWrite = view
	}

	if _, err := out.Write(toWrite); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	if _, err := out.Write([]byte(r.buildPostfix())); err != nil {
		return fmt.Errorf("write postfix: %w", err)
	}

	return nil
}

func (r Runner) Run(files []string, out io.Writer) error {
	var counter *byteCounter
	writer := out

	// If token estimation is requested, wrap the output writer
	// to count the actual bytes written (headers + content + footers).
	if r.ShowTokens {
		counter = &byteCounter{w: out}
		writer = counter
	}

	for _, path := range files {
		if err := r.runFile(path, writer); err != nil {
			return fmt.Errorf("lx: %w", err)
		}
	}

	if r.ShowTokens && counter != nil {
		// Heuristic: ~4 characters (bytes) per token.
		estimate := counter.total / 4
		fmt.Fprintf(os.Stderr, "Estimated Tokens: %d\n", estimate)
	}

	return nil
}

// byteCounter wraps an io.Writer to track the total number of bytes written.
type byteCounter struct {
	w     io.Writer
	total int64
}

func (b *byteCounter) Write(p []byte) (int, error) {
	n, err := b.w.Write(p)
	b.total += int64(n)
	return n, err
}
