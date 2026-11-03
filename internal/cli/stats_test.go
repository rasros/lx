package cli

import (
	"strings"
	"testing"

	"github.com/rasros/lx/internal/ansi"
)

// The token count turns yellow, then red, as it nears and passes common
// context sizes, and is plain without color.
func TestStatsFuncs_TokenLabel(t *testing.T) {
	tokenLabel := statsFuncs()["tokenLabel"].(func(int64, bool) string)

	if got := tokenLabel(1234, false); got != "~1,234 tokens" {
		t.Errorf("tokenLabel(1234,false) = %q, want %q", got, "~1,234 tokens")
	}
	cases := []struct {
		n     int64
		color string
	}{
		{tokenWarnThreshold - 1, string(ansi.Green)},
		{tokenWarnThreshold, string(ansi.Yellow)},
		{tokenDangerThreshold - 1, string(ansi.Yellow)},
		{tokenDangerThreshold, string(ansi.Red)},
	}
	for _, c := range cases {
		out := tokenLabel(c.n, true)
		if !strings.HasPrefix(out, c.color+string(ansi.Bold)) || !strings.HasSuffix(out, string(ansi.Reset)) {
			t.Errorf("tokenLabel(%d,true) = %q, want it %q, bold and reset", c.n, out, c.color)
		}
	}
}
