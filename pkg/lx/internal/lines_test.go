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

func TestLineNumberFormatterStringMatchesFormat(t *testing.T) {
	lnf := LineNumberFormatter{
		Head:      []byte("alpha\nbeta\n"),
		Gap:       []byte("... (7 rows skipped)\n"),
		Tail:      []byte("yankee\nzulu\n"),
		TotalRows: 11,
	}

	viaFormat := fmt.Sprintf("%v", lnf)
	if got := lnf.String(); got != viaFormat {
		t.Errorf("String() = %q, want it to match Format output %q", got, viaFormat)
	}
}

func TestLineNumberFormatterStringNumbersLines(t *testing.T) {
	lnf := LineNumberFormatter{Head: []byte("one\ntwo\n"), TotalRows: 2}
	want := "1: one\n2: two\n"
	if got := lnf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
