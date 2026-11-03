package internal

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestLineNumberFormatter(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:      []byte("one\n"),
		Gap:       []byte("...\n"),
		Tail:      []byte("ten\n"),
		TotalRows: 10,
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v", lnf)
	got := buf.String()

	if !strings.Contains(got, " 1: one") {
		t.Error("Missing single digit padded line number")
	}
	if !strings.Contains(got, "10: ten") {
		t.Error("Missing double digit line number")
	}
}

func TestLineNumberFormatterStringNumbersLines(t *testing.T) {
	lnf := LineNumberFormatter{Head: []byte("one\ntwo\n"), TotalRows: 2}
	want := "1: one\n2: two\n"
	if got := lnf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLineNumberFormatterExplicitRows(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:      []byte("func A() {\n...\nfunc B() {\n"),
		HeadRows:  []int{6, 0, 11},
		Gap:       []byte("... (20 rows skipped)\n"),
		Tail:      []byte("func C() {\n...\nfunc D() {\n"),
		TailRows:  []int{32, 0, 104},
		TotalRows: 120,
		// Explicit numbers are exact, so no "~" even for an estimated total.
		Approximate: true,
	}
	want := "  6: func A() {\n" +
		"...\n" +
		" 11: func B() {\n" +
		"... (20 rows skipped)\n" +
		" 32: func C() {\n" +
		"...\n" +
		"104: func D() {\n"
	if got := lnf.String(); got != want {
		t.Errorf("String() =\n%s\nwant\n%s", got, want)
	}
}

func TestLineNumberFormatterExplicitRowsSetWidth(t *testing.T) {
	// A row past TotalRows still gets room for all its digits.
	lnf := LineNumberFormatter{Head: []byte("a\nb\n"), HeadRows: []int{9, 10}, TotalRows: 9}
	if got, want := lnf.String(), " 9: a\n10: b\n"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
