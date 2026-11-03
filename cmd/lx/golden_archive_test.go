package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

// runTestGoldenHost runs cases in workDir, as runTestGoldenEnv does, naming
// host, a test server's address, as HOST, so the URLs in the output keep
// the form they were written in.
func runTestGoldenHost(t *testing.T, workDir, host string, env map[string]string, cases []goldenTestCase) {
	t.Helper()
	canonical, _ := filepath.EvalSymlinks(workDir)
	runGoldenTests(t, cases, goldenRun{cwd: workDir, env: env, scrub: []string{workDir, canonical}, hosts: []string{host}})
}

// fileServer serves dir until the test and its parallel cases are done,
// and returns its URL and its host.
func fileServer(t *testing.T, dir string) (url, host string) {
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(server.Close)
	return server.URL, strings.TrimPrefix(server.URL, "http://")
}

// An archive entry is no file on disk: it has no AbsPath, but names its
// archive, by path or by the URL it was downloaded from, and its path
// inside it. dataURI, which reads a file on disk, fails clearly for it.
func TestGoldenArchiveOrigin(t *testing.T) {
	dir := gf.SetupArchiveOriginFixture(t)
	url, host := fileServer(t, dir)

	runTestGoldenHost(t, dir, host, nil, []goldenTestCase{
		{name: "189a_archive_entry_origin", args: []string{"-y", "configs/origin.yaml", "-Z", "archive.zip", "notes.txt.gz"}},
		{name: "189b_url_archive_entry_origin", args: []string{"-y", "configs/origin.yaml", "-Z", url + "/archive.tar.gz"}},
		{name: "189c_archive_entry_datauri_fails", args: []string{"-y", "configs/datauri.yaml", "-Z", "archive.zip"}},
	})
}

// Archive entries over -m are skipped before they are read, and counted as
// skipped by size as any other file is. The size of a compressed file's
// content is unknown until it is read, so reading stops past the limit.
func TestGoldenArchiveMaxSize(t *testing.T) {
	dir := setupArchiveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "189d_max_size_archive_stats", args: []string{"--stats", "-Z", "-m", "16", "archive.zip", "archive.tar.gz"}},
		{name: "189e_max_size_single_file_gz", args: []string{"--stats", "-Z", "-m", "10", "notes.txt.gz"}},
	})
}

// Archives that misbehave are read as far as they can be. An entry whose
// path leaves the archive is skipped. An entry that ends short of what it
// claims fails, once, as any entry that cannot be read does, without lx
// trusting the claim with memory; when that ends a tar archive, the
// archive's own failure is the same one and is not counted again. The content of a compressed URL is named after the URL, not
// the file it was downloaded to. An archive is read as its content says,
// whatever its name says.
func TestGoldenArchiveEdges(t *testing.T) {
	dir := gf.SetupArchiveEdgeFixture(t)
	url, host := fileServer(t, dir)

	runTestGoldenHost(t, dir, host, nil, []goldenTestCase{
		{name: "189e2_archive_entries_outside_skipped", args: []string{"-n0", "-Z", "escape.tar"}},
		{name: "189e3_archive_entries_outside_skipped_tree", args: []string{"-t", "-Z", "escape.tar"}},
		{name: "189e4_tar_entry_claiming_1tb", args: []string{"-n0", "-Z", "lie.tar"}},
		{name: "189e5_zip_entry_claiming_4gb", args: []string{"-n0", "-Z", "lie.zip"}},
		{name: "189e8_zip_entries_claiming_more_each_fail", args: []string{"--stats", "-n0", "-Z", "lie3.zip"}},
		{name: "189e6_url_single_file_gz_named_after_url", args: []string{"-y", "configs/origin.yaml", "-Z", url + "/s100.txt.gz"}},
		{name: "189e7_archive_identified_by_content", args: []string{"-n0", "-Z", "zip-inside.tar.gz", "zip-inside.gz", "plain-tar.tar.gz"}},
	})
}
