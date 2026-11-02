package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

// -i patterns with a slash match paths relative to the walked directory, and
// the walk prunes the directories they cannot match below. The entries of an
// archive found in the walk are matched as files below the archive's path,
// so pruning and matching agree; a named archive, like a named directory, is
// matched from its own root.
func TestGoldenIncludePaths(t *testing.T) {
	dir := gf.SetupIncludePathsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "037b_include_wildcard_in_dir_name", args: []string{"-n0", "-i", "src*/x.go", "."}},
		{name: "037c_include_escaped_dir_name", args: []string{"-n0", "-i", `lit\[1\]/a.go`, "."}},
		{name: "037d_include_named_dir", args: []string{"-n0", "-i", "src/*.go", "proj"}},
		{name: "112f_walked_archive_include_from_walk_root", args: []string{"-n0", "-Z", "-i", "src/*.go", "."}},
		{name: "112g_walked_archive_include_wildcard_dir", args: []string{"-n0", "-Z", "-i", "src*/*.go", "."}},
		{name: "112h_walked_archive_include_archive_path", args: []string{"-n0", "-Z", "-i", "arch/a.zip/src/*.go", "."}},
		{name: "112i_walked_archive_include_floating", args: []string{"-n0", "-Z", "-i", "*.go", "."}},
		{name: "112j_named_archive_include_from_archive_root", args: []string{"-n0", "-Z", "-i", "src/*.go", "arch/a.zip"}},
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
