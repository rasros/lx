package cli

import (
	"testing"
)

func TestParseSizeLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1", 1},
		{"4096", 4096},
		{"512k", 512_000},
		{"512K", 512_000},
		{"512kb", 512_000},
		{"512KB", 512_000},
		{"2m", 2_000_000},
		{"2M", 2_000_000},
		{"2mb", 2_000_000},
		{"1g", 1_000_000_000},
		{"1G", 1_000_000_000},
		{"1GB", 1_000_000_000},
		{"900b", 900},
		{"900B", 900},
		{"  512k  ", 512_000},
	}

	for _, c := range cases {
		got, err := parseSizeLimit(c.in)
		if err != nil {
			t.Errorf("parseSizeLimit(%q) returned error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseSizeLimit(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseSizeLimitErrors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"notasize",
		"k",
		"kb",
		"0",
		"0k",
		"-1",
		"-5k",
		"1.5M",
		"512 k",
		"12x",
		"9223372036854775807k",
	}

	for _, in := range cases {
		if got, err := parseSizeLimit(in); err == nil {
			t.Errorf("parseSizeLimit(%q) = %d, want error", in, got)
		}
	}
}
