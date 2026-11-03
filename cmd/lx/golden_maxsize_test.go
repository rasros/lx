package main

import (
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

func TestGoldenMaxSize(t *testing.T) {
	// The slicing fixture holds src/large.txt (~1.6 kB) alongside several
	// files well under 100 bytes, so a limit between them is easy to express.
	dir := gf.SetupSlicingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "140_max_size_skips_large", args: []string{"-m", "1k", "."}},
		{name: "141_max_size_bytes_no_suffix", args: []string{"-m", "20", "."}},
		{name: "142_max_size_suffix_uppercase", args: []string{"-m", "2MB", "."}},
		{name: "143_max_size_tree_matches_content", args: []string{"-m", "1k", "-T", "."}},
		{name: "144_max_size_tree_only", args: []string{"-m", "1k", "-t", "."}},
		{name: "145_max_size_named_file", args: []string{"-m", "1k", "src/large.txt"}},
		{name: "147_max_size_per_section_override", args: []string{"-m", "20", ".", "-m", "2k", "src/large.txt"}},
		{name: "148_max_size_invalid_value", args: []string{"-m", "notasize", "."}},
		{name: "149_max_size_attached_short_form", args: []string{"-m1k", "."}},
		{name: "152_max_size_zero_rejected", args: []string{"-m", "0", "."}},
		{name: "153_max_size_skips_everything_warns", args: []string{"-m", "1", "src/large.txt"}},
		// stdin is a stream: read up to -m, and a failure past it.
		{name: "980_stdin_over_max_size_fails", args: []string{"-m", "10", "-"}, stdin: "0123456789abcdef\n"},
	})
}

func TestGoldenMaxSizeArchive(t *testing.T) {
	// archive.zip holds hello.txt (20 bytes) and nested/world.go (15 bytes).
	dir := gf.SetupArchiveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "150_max_size_archive_entries", args: []string{"-Z", "-m", "16", "archive.zip"}},
		{name: "151_max_size_archive_tree", args: []string{"-Z", "-m", "16", "-t", "archive.zip"}},
	})
}
