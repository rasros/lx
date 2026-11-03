package core

const (
	sampleThreshold = 256 * 1024
	sampleHead      = 128 * 1024
	sampleTail      = 64 * 1024
)

// DefaultTokenizer is the Tokenizer a Stream estimates with unless it is
// given another. It estimates BPE tokens from content with a character-class
// scanner and no vocabulary. Density follows the share of symbols: about 3.8
// bytes/token for prose, 3.2 for source, 2.0 for JSON.
func DefaultTokenizer(size int64, content interface{}) int64 {
	switch v := content.(type) {
	case string:
		return estimate(v, size)
	case []byte:
		return estimate(v, size)
	}
	return size / 4
}

type byteseq interface {
	~string | ~[]byte
}

// estimate scans the whole content when it is small enough, and otherwise
// extrapolates from a head and tail sample.
func estimate[T byteseq](d T, size int64) int64 {
	n := int64(len(d))
	if n == 0 {
		return size / 4
	}
	if n <= sampleThreshold {
		return scan(d)
	}

	headEnd := sampleHead
	tailStart := len(d) - sampleTail
	if tailStart < headEnd {
		return scan(d)
	}

	sampleTokens := scan(d[:headEnd]) + scan(d[tailStart:])
	sampleBytes := int64(headEnd + sampleTail)
	return sampleTokens * n / sampleBytes
}

func scan[T byteseq](d T) int64 {
	var (
		wordRuns, wordChars, symbols, wsRuns, newlines, nonAscii int64
		inWord, inWS                                             bool
	)
	for i := 0; i < len(d); i++ {
		c := d[i]
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
