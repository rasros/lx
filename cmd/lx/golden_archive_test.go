package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
	"github.com/rasros/lx/pkg/lx/sources"
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

// repoServer serves dir, with the repository URL host/owner/name of its
// host fetched as dir/name.zip. The registry of repository hosts is global, but each
// server has a host of its own and the registry is locked, so parallel
// goldens cannot meet there. Their cases run after the test function
// returns, so the host is removed on cleanup.
func repoServer(t *testing.T, dir string) (url, host string) {
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(server.Close)
	t.Cleanup(sources.SetRepoHostForTest(strings.TrimPrefix(server.URL, "http://"), func(owner, repo, ref string) string {
		return server.URL + "/" + repo + ".zip"
	}))
	return server.URL, strings.TrimPrefix(server.URL, "http://")
}

// A repository URL is read as a checkout of the repository from its host's
// archive, whose files are all below one "repo-main/" directory. They are
// displayed below the URL as written, without that directory, filtered by
// the repository's ignore files at every depth, and matched by the globs
// from the repository's root. An archive named as one, by URL or path, is
// expanded as it is, wrapper and ignore files included.
func TestGoldenRepoURL(t *testing.T) {
	dir := gf.SetupRepoArchiveFixture(t)
	url, host := repoServer(t, dir)
	repo := host + "/owner/repo"

	runTestGoldenHost(t, dir, host, nil, []goldenTestCase{
		{name: "189f_repo_url_entries_below_url", args: []string{"-n0", repo}},
		{name: "189g_repo_url_exclude_from_repo_root", args: []string{"-n0", "-e", "docs/**", repo}},
		{name: "189h_repo_url_include_from_repo_root", args: []string{"-n0", "-i", "internal/cli/*.go", repo}},
		{name: "189i_repo_url_include_floating", args: []string{"-n0", "-i", "*.md", "-e", "docs/deep/", repo}},
		{name: "189j_repo_url_glob_with_url_misses", args: []string{"-n0", "-i", repo + "/docs/*.md", repo}},
		{name: "189k_repo_url_hidden", args: []string{"-n0", "-H", "-i", ".github/**", "http://" + repo + "/"}},
		{name: "189l_repo_url_tree", args: []string{"-t", repo}},
		{name: "189u_repo_url_no_ignore", args: []string{"-n0", "--no-ignore", repo}},
		{name: "189w_url_archive_reads_no_ignore_files", args: []string{"-n0", "-Z", url + "/repo.zip"}},
		{name: "189x_archive_reads_no_ignore_files", args: []string{"-n0", "-Z", "repo.zip"}},
		{name: "189y_repo_url_entry_keeps_wrapper", args: []string{"-y", "configs/origin.yaml", "-i", "docs/*.md", repo}},
		{name: "189y2_repo_url_no_shared_wrapper", args: []string{"-n0", host + "/owner/nowrap"}},
		{name: "189y3_repo_url_entries_no_checkout_holds", args: []string{"-n0", host + "/owner/malformed"}},
	})
}

// The user's lx ignore file and git's global excludes apply to a repository
// URL as to a checkout, and --no-ignore lifts them with the rest.
func TestGoldenRepoURLUserIgnores(t *testing.T) {
	dir := gf.SetupRepoArchiveFixture(t)
	_, host := repoServer(t, dir)
	config := t.TempDir()
	for name, rules := range map[string]string{"lx/ignore": "README.md\n", "git/ignore": "internal/other/\n"} {
		p := filepath.Join(config, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rules), 0644); err != nil {
			t.Fatal(err)
		}
	}
	repo := host + "/owner/repo"
	runTestGoldenHost(t, dir, host, map[string]string{"XDG_CONFIG_HOME": config}, []goldenTestCase{
		{name: "189v_repo_url_user_and_global_ignores", args: []string{"-n0", repo}},
		{name: "189va_repo_url_user_and_global_ignores_no_ignore", args: []string{"-n0", "--no-ignore", repo}},
	})
}

// The content of a compressed file is its one entry, named without the
// compression suffix and matched below the file as any archive's entries
// are. The content of a named one is never skipped as hidden.
func TestGoldenSingleFileCompressionEntry(t *testing.T) {
	dir := gf.SetupRepoArchiveFixture(t)
	url, host := repoServer(t, dir)

	runTestGoldenHost(t, dir, host, nil, []goldenTestCase{
		{name: "189m_single_file_gz_exclude", args: []string{"-n0", "-Z", "-e", "*.txt", "notes.txt.gz"}},
		{name: "189m2_single_file_gz_exclude_other", args: []string{"-n0", "-Z", "-e", "*.md", "notes.txt.gz"}},
		{name: "189n_single_file_gz_include_below_archive", args: []string{"-n0", "-Z", "-i", "notes.txt.gz/*.txt", "notes.txt.gz"}},
		{name: "189n2_single_file_gz_include_archive_misses", args: []string{"-n0", "-Z", "-i", "*.gz", "notes.txt.gz"}},
		{name: "189n3_single_file_gz_include_archive_content", args: []string{"-n0", "-Z", "-i", "**/*.gz/*", "notes.txt.gz"}},
		{name: "189o_url_single_file_gz", args: []string{"-n0", "-Z", url + "/notes.txt.gz"}},
		{name: "189p_named_hidden_single_file_gz", args: []string{"-n0", "-Z", ".bashrc.gz"}},
		{name: "189p2_url_hidden_single_file_gz", args: []string{"-n0", "-Z", url + "/.bashrc.gz"}},
		{name: "189p3_walked_hidden_single_file_gz_skipped", args: []string{"-n0", "-Z", "-i", "**/*.gz/*", "."}},
		{name: "189p4_plain_gzip_named_tar_gz", args: []string{"-n0", "-Z", "plain.tar.gz"}},
	})
}

// Globs anchored at the working directory treat walked archives at any
// depth as the directories of the same names: each pair of goldens is the
// same.
func TestGoldenArchiveAnchoredGlobsAtDepth(t *testing.T) {
	arch, dirs := gf.SetupArchiveDepthFixture(t)
	excludes := []string{"-n0", "-Z", "-e", "d1/d2/low.zip/src/a.go", "-e", "top.zip/deep/", "-e", "d1/*/src/", "."}
	includes := []string{"-n0", "-Z", "-i", "d1/**/src/a.go", "-e", "d1/mid.zip/deep/**", "."}
	// The two trees are two working directories, so two batches.
	t.Run("archives", func(t *testing.T) {
		runTestGolden(t, arch, []goldenTestCase{
			{name: "189q_archive_anchored_excludes_at_depth", args: excludes},
			{name: "189s_archive_anchored_includes_at_depth", args: includes},
		})
	})
	t.Run("dirs", func(t *testing.T) {
		runTestGolden(t, dirs, []goldenTestCase{
			{name: "189r_dir_anchored_excludes_at_depth", args: excludes},
			{name: "189t_dir_anchored_includes_at_depth", args: includes},
		})
	})
}
