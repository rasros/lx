// Package ansi is the only place lx writes terminal escape sequences. Every
// styled string goes through a Painter, which emits plain text when color is
// off, so output respects NO_COLOR, TERM and redirection everywhere.
package ansi

import (
	"io"
	"os"
)

// Code is an SGR escape sequence.
type Code string

const (
	Reset       Code = "\033[0m"
	Bold        Code = "\033[1m"
	Dim         Code = "\033[2m"
	Underline   Code = "\033[4m"
	NoIntensity Code = "\033[22m"
	NoUnderline Code = "\033[24m"
	Red         Code = "\033[31m"
	Green       Code = "\033[32m"
	Yellow      Code = "\033[33m"
	Cyan        Code = "\033[36m"
)

// Painter applies styles when enabled and returns text unchanged otherwise.
type Painter struct {
	enabled bool
}

// New returns a Painter that styles text only when enabled is true.
func New(enabled bool) Painter { return Painter{enabled: enabled} }

// For returns a Painter for output written to w, see Enabled.
func For(w io.Writer) Painter { return New(Enabled(w)) }

// Enabled reports whether output written to w should be colored. NO_COLOR,
// CLICOLOR=0 and an empty or dumb TERM disable color; otherwise
// CLICOLOR_FORCE or FORCE_COLOR enable it, and without them only a terminal
// on stdout or stderr gets color.
func Enabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" || os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	var f *os.File
	switch w {
	case os.Stderr:
		f = os.Stderr
	case os.Stdout:
		f = os.Stdout
	default:
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// Enabled reports whether p styles text.
func (p Painter) Enabled() bool { return p.enabled }

// Paint applies codes to s and resets all attributes after it.
func (p Painter) Paint(s string, codes ...Code) string {
	if !p.enabled || len(codes) == 0 {
		return s
	}
	n := len(s) + len(Reset)
	for _, c := range codes {
		n += len(c)
	}
	b := make([]byte, 0, n)
	for _, c := range codes {
		b = append(b, c...)
	}
	b = append(b, s...)
	b = append(b, Reset...)
	return string(b)
}

// Wrap surrounds s with open and close, for styles that must end without
// resetting attributes set around them, such as bold inside underline.
func (p Painter) Wrap(s string, open, close Code) string {
	if !p.enabled {
		return s
	}
	return string(open) + s + string(close)
}

// Code returns c when enabled and an empty string otherwise, for building
// output piece by piece.
func (p Painter) Code(c Code) string {
	if !p.enabled {
		return ""
	}
	return string(c)
}
