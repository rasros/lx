package lx

import "fmt"

// TokenCounter allows plugging in different LLM tokenizers.
// It accepts the file size and the rendered content (string or bytes) to estimate tokens.
type TokenCounter func(size int64, content interface{}) int64

// Tokenizer defines the interface for estimating LLM tokens.
// Implement this interface to provide custom token counting logic (e.g. tiktoken).
type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}

// DefaultTokenCounter provides a simple 4-char-per-token heuristic.
func DefaultTokenCounter(size int64, content interface{}) int64 {
	var targetSize int64 = size
	switch v := content.(type) {
	case string:
		targetSize = int64(len(v))
	case []byte:
		targetSize = int64(len(v))
	case fmt.Stringer:
		targetSize = int64(len(v.String()))
	}
	// A crude but standard approximation: 1 token ~= 4 characters (English).
	return targetSize / 4
}
