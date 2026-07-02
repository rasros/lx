package core

import "fmt"

const (
	sampleThreshold = 256 * 1024
	sampleHead      = 128 * 1024
	sampleTail      = 64 * 1024
)

// DefaultTokenCounter estimates BPE tokens from content using a character-class
// scanner. Auto-adapts to content shape (prose ~bytes/4, code ~bytes/3.2,
// JSON ~bytes/2.7) without needing a language hint or embedded vocabulary.
func DefaultTokenCounter(size int64, content interface{}) int64 {
	switch v := content.(type) {
	case string:
		return estimateString(v, size)
	case []byte:
		return estimateBytes(v, size)
	case fmt.Stringer:
		return estimateString(v.String(), size)
	}
	return size / 4
}

func estimateString(s string, size int64) int64 {
	n := int64(len(s))
	if n == 0 {
		return size / 4
	}
	if n <= sampleThreshold {
		return scanString(s)
	}
	headEnd := sampleHead
	tailStart := len(s) - sampleTail
	if tailStart < headEnd {
		return scanString(s)
	}
	sampleTokens := scanString(s[:headEnd]) + scanString(s[tailStart:])
	sampleBytes := int64(headEnd + sampleTail)
	return sampleTokens * n / sampleBytes
}

func estimateBytes(b []byte, size int64) int64 {
	n := int64(len(b))
	if n == 0 {
		return size / 4
	}
	if n <= sampleThreshold {
		return scanBytes(b)
	}
	headEnd := sampleHead
	tailStart := len(b) - sampleTail
	if tailStart < headEnd {
		return scanBytes(b)
	}
	sampleTokens := scanBytes(b[:headEnd]) + scanBytes(b[tailStart:])
	sampleBytes := int64(headEnd + sampleTail)
	return sampleTokens * n / sampleBytes
}

func scanString(s string) int64 {
	var (
		wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii int64
		inWord, inWS                                             bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 0x80:
			nonAscii++
			inWord, inWS = false, false
		case isWord(c):
			if !inWord {
				wordRuns++
				inWord = true
			}
			wordChars++
			inWS = false
		case c == '\n':
			newlines++
			inWord, inWS = false, false
		case isWS(c):
			if !inWS {
				wsRuns++
				inWS = true
			}
			inWord = false
		default:
			symbols++
			inWord, inWS = false, false
		}
	}
	return combine(wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii)
}

func scanBytes(b []byte) int64 {
	var (
		wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii int64
		inWord, inWS                                             bool
	)
	for _, c := range b {
		switch {
		case c >= 0x80:
			nonAscii++
			inWord, inWS = false, false
		case isWord(c):
			if !inWord {
				wordRuns++
				inWord = true
			}
			wordChars++
			inWS = false
		case c == '\n':
			newlines++
			inWord, inWS = false, false
		case isWS(c):
			if !inWS {
				wsRuns++
				inWS = true
			}
			inWord = false
		default:
			symbols++
			inWord, inWS = false, false
		}
	}
	return combine(wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii)
}

func isWord(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func isWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f'
}

// combine implements: wordRuns + max(0,(wordChars-wordRuns*4)/4)
//   - 0.55*symbols + 0.30*wsRuns + newlines + 0.50*nonAscii
//
// Done in integer arithmetic via *100 / 100.
func combine(wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii int64) int64 {
	extraWord := wordChars - wordRuns*4
	if extraWord < 0 {
		extraWord = 0
	}
	hundredths := wordRuns*100 +
		(extraWord*100)/4 +
		symbols*55 +
		wsRuns*30 +
		newlines*100 +
		nonAscii*50
	return hundredths / 100
}
