package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

// -i patterns with a slash match paths from the working directory, and the
// walk prunes the directories they cannot match below. The entries of an
// expanded archive, found in the walk or named, are matched as the paths
// below the archive's own, so pruning and matching agree.
func TestGoldenIncludePaths(t *testing.T) {
	dir := gf.SetupIncludePathsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "037b_include_wildcard_in_dir_name", args: []string{"-n0", "-i", "src*/x.go", "."}},
		{name: "037c_include_escaped_dir_name", args: []string{"-n0", "-i", `lit\[1\]/a.go`, "."}},
		{name: "037d_include_named_dir", args: []string{"-n0", "-i", "src/*.go", "proj"}},
		{name: "037e_include_double_star_in_braces", args: []string{"-n0", "-i", "proj/{**,x}/c.go", "."}},
		{name: "112f_walked_archive_include_from_cwd", args: []string{"-n0", "-Z", "-i", "src/*.go", "."}},
		{name: "112g_walked_archive_include_wildcard_dir", args: []string{"-n0", "-Z", "-i", "src*/*.go", "."}},
		{name: "112h_walked_archive_include_archive_path", args: []string{"-n0", "-Z", "-i", "arch/a.zip/src/*.go", "."}},
		{name: "112i_walked_archive_include_floating", args: []string{"-n0", "-Z", "-i", "*.go", "."}},
		{name: "112j_named_archive_include_misses_archive_root", args: []string{"-n0", "-Z", "-i", "src/*.go", "-i", "**/notes.txt", "arch/a.zip"}},
		{name: "112l_walked_archive_include_double_star", args: []string{"-n0", "-Z", "-i", "arch/a.zip/**", "."}},
		{name: "112m_walked_archive_exclude_from_cwd", args: []string{"-n0", "-Z", "-i", "arch/a.zip/**", "-e", "arch/a.zip/src/a.go", "."}},
		{name: "112n_walked_archive_exclude_from_cwd_in_named_dir", args: []string{"-n0", "-Z", "-e", "arch/a.zip/src/a.go", "arch"}},
		{name: "112o_named_archive_exclude_misses_archive_root", args: []string{"-n0", "-Z", "-e", "src/a.go", "arch/a.zip"}},
	})
}

// A URL archive that cannot be downloaded is reported and counted as failed.
func TestGoldenURLArchiveDownloadFailure(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	runTestGolden(t, dir, []goldenTestCase{
		{name: "112k_url_archive_download_failure", args: []string{"--stats", "-Z", server.URL + "/missing.zip"}},
	}, server.URL)
}
