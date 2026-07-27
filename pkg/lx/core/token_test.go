package core

import (
	"strings"
	"testing"
)

type mockStringer string

func (m mockStringer) String() string { return string(m) }

func TestDefaultTokenCounter_Basic(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		content interface{}
		want    int64
	}{
		{"single word 4 chars", 4, "1234", 1},
		{"single word 8 chars splits", 8, "12345678", 2},
		{"short word still costs 1", 3, "abc", 1},
		{"stringer", 100, mockStringer("1234"), 1},
		{"nil falls back to size/4", 12, nil, 3},
		{"empty string falls back to size/4", 16, "", 4},
		{"unknown type falls back to size/4", 40, 12345, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultTokenCounter(tt.size, tt.content)
			if got != tt.want {
				t.Errorf("DefaultTokenCounter() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDefaultTokenCounter_CodeDenserThanProse(t *testing.T) {
	prose := strings.Repeat("the quick brown fox jumps over the lazy dog ", 20)
	code := strings.Repeat("if (x == y) { return foo(bar, baz); }\n", 20)

	proseTokens := DefaultTokenCounter(int64(len(prose)), prose)
	codeTokens := DefaultTokenCounter(int64(len(code)), code)

	proseRatio := float64(len(prose)) / float64(proseTokens)
	codeRatio := float64(len(code)) / float64(codeTokens)

	if proseRatio < 3.5 || proseRatio > 4.5 {
		t.Errorf("prose chars/token = %.2f, want ~4.0", proseRatio)
	}
	if codeRatio > 3.6 {
		t.Errorf("code chars/token = %.2f, want < 3.6 (denser than prose)", codeRatio)
	}
}

func TestDefaultTokenCounter_JSONDenseSymbols(t *testing.T) {
	json := strings.Repeat(`{"a":1,"b":[true,false,null],"c":"x"},`, 20)
	tokens := DefaultTokenCounter(int64(len(json)), json)
	ratio := float64(len(json)) / float64(tokens)
	if ratio > 3.2 {
		t.Errorf("json chars/token = %.2f, want < 3.2", ratio)
	}
}

func TestDefaultTokenCounter_LongIdentifierSplits(t *testing.T) {
	short := DefaultTokenCounter(4, "abcd")
	long := DefaultTokenCounter(40, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJ1234")
	if long <= short {
		t.Errorf("long identifier (%d) should split into more tokens than short (%d)", long, short)
	}
	if long < 5 {
		t.Errorf("40-char identifier produced only %d tokens, want >= 5", long)
	}
}

func TestDefaultTokenCounter_NonAsciiSurcharge(t *testing.T) {
	ascii := "hello world hello world"
	utf8 := "héllo wörld héllo wörld"
	a := DefaultTokenCounter(int64(len(ascii)), ascii)
	u := DefaultTokenCounter(int64(len(utf8)), utf8)
	if u <= a {
		t.Errorf("utf-8 string (%d tokens) should cost >= ascii equivalent (%d tokens)", u, a)
	}
}

func TestDefaultTokenCounter_ByteSliceMatchesString(t *testing.T) {
	s := "package main\n\nfunc main() { println(\"hi\") }\n"
	fromString := DefaultTokenCounter(int64(len(s)), s)
	fromBytes := DefaultTokenCounter(int64(len(s)), []byte(s))
	if fromString != fromBytes {
		t.Errorf("string=%d bytes=%d, want equal", fromString, fromBytes)
	}
}

func TestDefaultTokenCounter_LargeContentSampling(t *testing.T) {
	// Build a >256 KiB code-like buffer so the sampling path triggers.
	unit := "func add(a, b int) int { return a + b }\n"
	var sb strings.Builder
	for sb.Len() < sampleThreshold+64*1024 {
		sb.WriteString(unit)
	}
	full := sb.String()

	sampled := DefaultTokenCounter(int64(len(full)), full)

	// Same content scanned without sampling: feed it as a slice that's exactly
	// the content but invoke the inner scanner directly via a sub-threshold split.
	exact := scan(full)

	diff := sampled - exact
	if diff < 0 {
		diff = -diff
	}
	tolerance := exact / 20 // 5%
	if diff > tolerance {
		t.Errorf("sampled=%d exact=%d diff=%d > tolerance=%d", sampled, exact, diff, tolerance)
	}
}

type stringerContent struct{ s string }

func (s stringerContent) String() string { return s.s }

func TestDefaultTokenCounterUsesStringerContent(t *testing.T) {
	content := stringerContent{s: strings.Repeat(`{"a":1},`, 90)}
	size := int64(len(content.s))

	got := DefaultTokenCounter(size, content)
	if got == size/4 {
		t.Errorf("estimate %d equals the size/4 fallback; Stringer content was not scanned", got)
	}
	if want := DefaultTokenCounter(size, content.s); got != want {
		t.Errorf("Stringer estimate %d != equivalent string estimate %d", got, want)
	}
}

// Absolute ratios drift with the sample, so this pins the ordering: density is
// driven by how much of the content is symbols rather than words. Long string
// values make JSON read almost like prose.
func TestDefaultTokenCounter_DensityFollowsSymbolShare(t *testing.T) {
	ratio := func(text string) float64 {
		return float64(len(text)) / float64(DefaultTokenCounter(int64(len(text)), text))
	}

	prose := ratio(strings.Repeat("the quick brown fox jumps over the lazy dog ", 20))
	code := ratio(strings.Repeat("func handleRequest(ctx context.Context) error {\n\treturn nil\n}\n", 20))
	jsonSymbols := ratio(strings.Repeat(`{"a":1,"b":[true,false,null],"c":"x"},`, 20))
	jsonWords := ratio(strings.Repeat(`{"email":"user42@example.com","name":"Alexander Hamilton"},`, 20))

	if prose < 3.4 || prose > 4.2 {
		t.Errorf("prose chars/token = %.2f, want between 3.4 and 4.2", prose)
	}
	if code < 2.9 || code > 3.5 {
		t.Errorf("code chars/token = %.2f, want between 2.9 and 3.5", code)
	}
	if jsonSymbols >= code {
		t.Errorf("symbol-dense JSON (%.2f) should be denser than code (%.2f)", jsonSymbols, code)
	}
	if jsonWords <= jsonSymbols {
		t.Errorf("JSON of long string values (%.2f) should be sparser than symbol-dense JSON (%.2f)",
			jsonWords, jsonSymbols)
	}
	if jsonWords >= prose {
		t.Errorf("JSON of long string values (%.2f) should still beat prose (%.2f)", jsonWords, prose)
	}
}

// The two representations shared duplicated scanners before; this pins that one
// implementation now serves both.
func TestDefaultTokenCounterAgreesAcrossRepresentations(t *testing.T) {
	samples := []string{
		"",
		"hello world",
		strings.Repeat(`{"a":1,"b":[true,false]},`, 40),
		strings.Repeat("func Alpha() int {\n\treturn 1\n}\n", 40),
		strings.Repeat("naïve café — résumé ", 40),
		strings.Repeat("x", sampleThreshold+sampleHead+sampleTail+1),
	}

	for i, s := range samples {
		size := int64(len(s))
		fromString := DefaultTokenCounter(size, s)
		fromBytes := DefaultTokenCounter(size, []byte(s))
		if fromString != fromBytes {
			t.Errorf("sample %d: string gave %d, []byte gave %d", i, fromString, fromBytes)
		}
	}
}
