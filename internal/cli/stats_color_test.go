package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestShouldColorStats(t *testing.T) {
	saved := map[string]string{}
	for _, k := range []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "TERM"} {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	pipe := &bytes.Buffer{}

	tests := []struct {
		name string
		env  map[string]string
		w    io.Writer
		want bool
	}{
		{"unknown writer with no force vars", nil, pipe, false},
		{"NO_COLOR blocks even with FORCE_COLOR", map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1", "TERM": "xterm"}, os.Stdout, false},
		{"CLICOLOR=0 blocks", map[string]string{"CLICOLOR": "0", "FORCE_COLOR": "1", "TERM": "xterm"}, os.Stdout, false},
		{"TERM=dumb blocks", map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, os.Stdout, false},
		{"empty TERM blocks (typical AI agent / CI env)", map[string]string{"FORCE_COLOR": "1"}, os.Stdout, false},
		{"CLICOLOR_FORCE on stdout overrides non-TTY", map[string]string{"CLICOLOR_FORCE": "1", "TERM": "xterm"}, os.Stdout, true},
		{"FORCE_COLOR on stderr overrides non-TTY", map[string]string{"FORCE_COLOR": "1", "TERM": "xterm"}, os.Stderr, true},
		{"plain TTY-less stdout, no force", map[string]string{"TERM": "xterm"}, os.Stdout, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "TERM"} {
				os.Unsetenv(k)
			}
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			got := shouldColorStats(tt.w)
			if got != tt.want {
				t.Errorf("shouldColorStats() = %v, want %v", got, tt.want)
			}
		})
	}
}
