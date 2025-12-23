package lx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
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
	// if token counting is NOT requested, use the standard efficient loop
	if !r.ShowTokens {
		for _, path := range files {
			if err := r.runFile(path, out); err != nil {
				return fmt.Errorf("lx: %w", err)
			}
		}
		return nil
	}

	// if token counting is requested, we must buffer the output first
	var buf bytes.Buffer
	for _, path := range files {
		// Write to our buffer instead of directly to 'out'
		if err := r.runFile(path, &buf); err != nil {
			return fmt.Errorf("lx: %w", err)
		}
	}

	// calculate and print token to Stderr
	fullOutput := buf.String()
	printTokenCount(fullOutput)

	// 2. Write the actual content to the original output
	if _, err := out.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write final output: %w", err)
	}

	return nil
}

// Helper function to count tokens using OpenAI's encoding
func printTokenCount(text string) {
	tkm, err := tiktoken.GetEncoding("cl100k_base") // standard for GPT-4 / GPT-3.5
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not calculate tokens: %v\n", err)
		return
	}

	tokens := tkm.Encode(text, nil, nil)
	fmt.Fprintf(os.Stderr, "Estimated Tokens: %d\n", len(tokens))
}
