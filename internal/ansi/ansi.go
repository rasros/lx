// Package ansi is the only place lx writes terminal escape sequences. Every
// styled string goes through a Painter, which emits plain text when color is
// off, so output respects NO_COLOR, TERM and redirection everywhere.
package ansi

import (
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/rasros/lx/internal/osenv"
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
func For(w io.Writer, env osenv.Lookup) Painter { return New(Enabled(w, env)) }

// Enabled reports whether output written to w should be colored, given the
// environment variables env looks up:
//
//   - NO_COLOR, CLICOLOR=0 or TERM=dumb turn color off;
//   - FORCE_COLOR turns it on, unless it is 0 or false, which turn it off;
//   - CLICOLOR_FORCE, unless it is 0, turns it on;
//   - otherwise only a terminal gets color, and outside Windows, whose
//     consoles never set TERM, only when TERM is set.
func Enabled(w io.Writer, env osenv.Lookup) bool {
	get := env.Get
	if get("NO_COLOR") != "" || get("CLICOLOR") == "0" {
		return false
	}
	term := get("TERM")
	if term == "dumb" {
		return false
	}
	if force, ok := env("FORCE_COLOR"); ok {
		return force != "0" && !strings.EqualFold(force, "false")
	}
	if force := get("CLICOLOR_FORCE"); force != "" && force != "0" {
		return true
	}
	if term == "" && runtime.GOOS != "windows" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a file attached to a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

// Enabled reports whether p styles text.
func (p Painter) Enabled() bool { return p.enabled }

// Paint applies codes to s and resets all attributes after it.
func (p Painter) Paint(s string, codes ...Code) string {
	if !p.enabled || len(codes) == 0 {
		return s
	}
	var b strings.Builder
	for _, c := range codes {
		b.WriteString(string(c))
	}
	b.WriteString(s)
	b.WriteString(string(Reset))
	return b.String()
}

// Wrap surrounds s with open and close, for styles that must end without
// resetting attributes set around them, such as bold inside underline.
func (p Painter) Wrap(s string, open, close Code) string {
	if !p.enabled {
		return s
	}
	return string(open) + s + string(close)
}
