package cli

import (
	"strings"
	"testing"
)

func TestPrintHelp(t *testing.T) {
	// Capture output
	out, err := captureStdout(func() error {
		printHelp()
		return nil
	})
	if err != nil {
		t.Fatalf("printHelp capture error: %v", err)
	}

	required := []string{
		"NAME:",
		"USAGE:",
		"GLOBAL OPTIONS:",
		"INTERLEAVED OPTIONS",
		"ACTIONS",
		"--head",
		"--tail",
		"--lines",
		"--copy",
	}

	for _, req := range required {
		if !strings.Contains(out, req) {
			t.Errorf("Help output missing %q", req)
		}
	}
}
