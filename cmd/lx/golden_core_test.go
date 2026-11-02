package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenWalk(t *testing.T) {
	dir := setupWalkFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "001_walk_default", args: []string{"."}},
		{name: "002_walk_compact", args: []string{"-n0", "."}},
		{name: "003_specific_file", args: []string{"main.go"}},
		{name: "004_multiple_roots", args: []string{"pkg", "README.md"}},
		{name: "005_walk_subdir", args: []string{"src"}},
		{name: "006_walk_parent_ref", args: []string{"src/../pkg"}},
		{name: "007_walk_dot_slash", args: []string{"./src"}},
	})
}

func TestGoldenFormatting(t *testing.T) {
	dir := setupFormattingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "010_fmt_xml", args: []string{"--xml", "."}},
		{name: "011_fmt_html", args: []string{"--html", "main.go", "README.md"}},
		{name: "012_fmt_markdown_explicit", args: []string{"--md", "main.go"}},
		{name: "013_fmt_xml_sections", args: []string{"--xml", "-s", "Core", "main.go", "-s", "Docs", "README.md"}},
		{name: "014_fmt_html_sections", args: []string{"--html", "-s", "Core", "main.go"}},
		{name: "015_fmt_markdown_no_fences", args: []string{"-y", "configs/custom_template.yaml", "main.go"}},
		{name: "016_fmt_bare", args: []string{"--bare", "main.go", "README.md"}},
	})
}

func TestGoldenSections(t *testing.T) {
	dir := setupSectionsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "020_sections_explicit", args: []string{"-s", "Docs", "doc", "-s", "Source", "src"}},
		{name: "021_prompts_mixed", args: []string{"-p", "Analyze this:", "main.go", "-p", "Determine bug"}},
		{name: "022_xml_complex_structure", args: []string{"--xml", "-s", "Context", "-p", "Read carefully", "README.md", "-s", "Code", "main.go"}},
		{name: "023_prompt_only", args: []string{"-p", "Just a question", "-p", "Another question"}},
		{name: "024_section_empty", args: []string{"-s", "Empty Section"}},
		{name: "025_interleaved_prompts_files", args: []string{"main.go", "-p", "Explain above", "pkg/util.go", "-p", "Explain below"}},
	})
}

func TestGoldenFiltering(t *testing.T) {
	dir := setupFilteringFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "030_include_only_go", args: []string{"-i", "*.go", "."}},
		{name: "031_exclude_tests", args: []string{"-e", "*_test.go", "."}},
		{name: "032_mixed_filters", args: []string{"-i", "*.go", "-e", "*_test.go", "."}},
		{name: "033_filter_reset", args: []string{"-i", "*.md", ".", "-i", "*.go", "."}},
		{name: "034_hidden_files", args: []string{"-H", "."}},
		{name: "035_no_ignore_files", args: []string{"--no-ignore", "."}},
		{name: "036_exclude_dir", args: []string{"-e", "src", "."}},
		{name: "037_include_path_match", args: []string{"-i", "src/script.py", "."}},
		{name: "038_nested_ignore_file", args: []string{"ignore_test"}},
		{name: "039_complex_globs", args: []string{"-i", "**/*.{go,py}", "-e", "**/main*", "."}},
	})
}

func TestGoldenSlicing(t *testing.T) {
	dir := setupSlicingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "040_lines_limit", args: []string{"--lines", "4", "."}},
		{name: "041_line_numbers", args: []string{"-l", "main.go"}},
		{name: "042_progressive_state", args: []string{
			"-s", "Raw", "README.md",
			"-s", "Numbered", "-l", "main.go",
			"-s", "Sliced", "-n", "2", "pkg/util.go",
		}},
		{name: "043_head_tail", args: []string{"--head", "3", "src/large.txt", "--tail", "2", "src/large.txt"}},
		{name: "044_tail_only", args: []string{"--tail", "5", "src/large.txt"}},
		{name: "045_head_only", args: []string{"--head", "5", "src/large.txt"}},
		{name: "046_head_tail_overlap", args: []string{"--head", "100", "--tail", "100", "src/large.txt"}},
		{name: "047_lines_0_compact", args: []string{"-n", "0", "main.go"}},
		{name: "048_slice_binary", args: []string{"--head", "5", "bin/data.bin"}},
	})
}

// Row counts and tail line numbers are exact past 32 KB, and CRLF line endings
// survive in head and tail alike.
func TestGoldenSlicingLarge(t *testing.T) {
	dir := setupLargeSlicingFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "040b_lines_large_line_numbers", args: []string{"--lines", "6", "-l", "big.txt"}},
		{name: "040c_lines_crlf", args: []string{"--lines", "4", "crlf.txt"}},
		{name: "040d_lines_crlf_line_numbers", args: []string{"--lines", "4", "-l", "crlf.txt"}},
		{name: "041b_line_numbers_crlf", args: []string{"-l", "crlf.txt"}},
		{name: "044b_tail_large", args: []string{"--tail", "4", "big.txt"}},
		{name: "044c_tail_large_line_numbers", args: []string{"--tail", "4", "-l", "big.txt"}},
		{name: "045b_head_large", args: []string{"--head", "3", "big.txt"}},
		{name: "046b_lines_overlap_crlf", args: []string{"--lines", "10", "crlf.txt"}},
		{name: "047b_lines_0_compact_large", args: []string{"-n", "0", "big.txt"}},
	})
}

func TestGoldenSymlinks(t *testing.T) {
	dir := setupSymlinksFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "049_symlinks_relative_target", args: []string{"rellinks"}},
		{name: "050_symlinks_default", args: []string{"links"}},
		{name: "051_symlinks_follow", args: []string{"--follow", "links"}},
		{name: "052_symlinks_dag", args: []string{"--follow", "links/loop"}},
		{name: "053_symlinks_infinite_cycle", args: []string{"--follow", "links/cycle_a"}},
		{name: "054_hidden_flag", args: []string{"-H", "links"}},
		{name: "055_no_file_links", args: []string{"--no-links", "links"}},
		{name: "056_follow_dirs_no_file_links", args: []string{"--follow", "--no-links", "links"}},
		{name: "057_broken_symlink", args: []string{"links/broken_link"}},
		{name: "058_symlinks_follow_include_go", args: []string{"--follow", "-i", "*.go", "links"}},
		{name: "059_symlinks_follow_hidden", args: []string{"--follow", "-H", "links"}},
	})
}

func TestGoldenErrors(t *testing.T) {
	dir := setupErrorsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "060_access_denied_file", args: []string{"secret/locked.txt"}},
		{name: "061_access_denied_dir", args: []string{"secret/locked_dir"}},
		{name: "062_missing_file", args: []string{"nonexistent.go"}},
		{name: "063_spaces_in_filename", args: []string{"spaces/file with spaces.txt"}},
		{name: "064_spaces_in_dirname", args: []string{"spaces"}},
		{name: "065_stdin_content", args: []string{"-"}, stdin: "Some content from pipe\nLine 2"},
		{name: "066_stdin_and_files", args: []string{"main.go", "-"}, stdin: "Piped content"},
		{name: "067_stdin_file_list", args: []string{}, stdin: "main.go\nREADME.md"},
		{name: "068_stdin_file_list_with_prompt", args: []string{"-p", "Review these"}, stdin: "main.go\nREADME.md"},
		{name: "069_stdin_file_list_with_system_context", args: []string{"-E"}, stdin: "main.go"},
	})
}

func TestGoldenStats(t *testing.T) {
	dir := setupStatsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "070_stats_forced", args: []string{"--stats", "main.go"}},
		{name: "071_quiet_mode", args: []string{"-q", "main.go"}},
		{name: "072_verbose_debug", args: []string{"-vv", "main.go"}},
		{name: "073_verbose_flag", args: []string{"--verbose=debug", "main.go"}},
		{name: "074_no_stats", args: []string{"--no-stats", "main.go"}},
		{name: "075_stats_auto_piped", args: []string{"main.go"}, autoStat: true},
		{name: "076_stats_counters_binary", args: []string{"--stats", "main.go", "data.bin"}},
		{name: "077_stats_counters_size_skip", args: []string{"--stats", "-m", "1k", "."}},
		{name: "078_stats_no_counters_when_clean", args: []string{"--stats", "main.go"}},
		{name: "079_stats_counts_unresolved_path", args: []string{"--stats", "main.go", "nope.go"}},
	})
}

func TestGoldenDetection(t *testing.T) {
	dir := setupDetectionFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "080_binary_detection", args: []string{"bin/data.bin"}},
		{name: "081_empty_file", args: []string{"bin/empty.txt"}},
		{name: "082_lang_detection_rs", args: []string{"langs/main.rs"}},
		{name: "083_lang_detection_py", args: []string{"src/script.py"}},
		{name: "084_lang_detection_dockerfile", args: []string{"langs/Dockerfile"}},
		{name: "085_lang_detection_shebang", args: []string{"langs/script_no_ext"}},
		{name: "086_render_image_md", args: []string{"assets/logo.png"}},
		{name: "087_render_image_html", args: []string{"--html", "assets/logo.png"}},
		{name: "088_render_image_compact", args: []string{"-n0", "assets/logo.png"}},
		{name: "089_html_escapes_markup", args: []string{"--html", "langs/page.html"}},
		{name: "089b_html_escapes_markup_line_numbers", args: []string{"--html", "-l", "langs/page.html"}},
	})
}

func TestGoldenConfig(t *testing.T) {
	dir := setupConfigFixture(t)
	copyCfgDir := t.TempDir()
	copyCfgPath := filepath.Join(copyCfgDir, "copy.yaml")
	if err := os.WriteFile(copyCfgPath, []byte("output_mode: copy\n"), 0644); err != nil {
		t.Fatalf("write copy config: %v", err)
	}
	runTestGolden(t, dir, []goldenTestCase{
		{name: "090_config_hidden", args: []string{"-H", "."}},
		{name: "091_config_follow", args: []string{"--follow", "links"}},
		{name: "092_config_combined", args: []string{"-H", "--follow", "links"}},
		{name: "093_named_hidden_file", args: []string{".gitignore"}},
		{name: "094_exclude_does_not_filter_named_file", args: []string{"-e", "*.go", "main.go"}},
		{name: "095_config_custom_sections", args: []string{"-y", "configs/custom_sections.yaml", "-s", "Head", "main.go"}},
		{name: "096_direct_file_parent_ignore", args: []string{"direct_ignore_test/test.ignored", "direct_ignore_test/test.kept"}},
		{name: "097_stdout_overrides_copy_mode", args: []string{"--stdout", "-y", copyCfgPath, "main.go"}},
	})
}

func TestGoldenComplex(t *testing.T) {
	dir := setupComplexFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "100_complex_review_bundle", args: []string{
			"--xml",
			"-p", "Review this code",
			"-s", "Configuration", "-i", "*.yaml", ".",
			"-s", "Source", "-i", "*.go", "-e", "*_test.go", ".",
			"-s", "Tests", "-i", "*_test.go", ".",
		}},
		{name: "101_stdin_null_terminated", args: []string{"-0"}, stdin: "main.go\x00README.md\x00spaces/file with spaces.txt\x00"},
		{name: "102_walk_nested_respects_root_ignore", args: []string{"parent_ignore_test/level1/level2"}},
		{name: "103_complex_ignore_exceptions", args: []string{"ignore_exception_test"}},
	})
}

func TestGoldenArchive(t *testing.T) {
	dir := setupArchiveFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "110_expand_archive_zip", args: []string{"-Z", "archive.zip"}},
		{name: "111_expand_archive_no_flag", args: []string{"archive.zip"}},
		{name: "112_expand_archive_filter", args: []string{"-Z", "-i", "*.go", "archive.zip"}},
		{name: "112b_expand_archive_exclude_glob", args: []string{"-Z", "-e", "*.txt", "archive.zip"}},
		{name: "112c_expand_archive_exclude_dir", args: []string{"-Z", "-e", "nested/", "archive.tar.gz"}},
		{name: "112e_expand_archive_exclude_in_dir_walk", args: []string{"-Z", "-e", "*.txt", "-e", "*.gz", "-e", "*.bz2", "-e", "*.tar", "-e", "images.zip", "."}},
		{name: "113_expand_archive_hidden", args: []string{"-Z", "-H", "archive.zip"}},
		{name: "114_expand_archive_line_numbers", args: []string{"-Z", "-l", "archive.zip"}},
		{name: "115_expand_archive_tar_gz", args: []string{"-Z", "archive.tar.gz"}},
		{name: "116_expand_archive_xml", args: []string{"--xml", "-Z", "archive.zip"}},
		{name: "117_expand_archive_hidden_filter_go", args: []string{"-Z", "-H", "-i", "*.go", "archive.zip"}},
		{name: "118_expand_archive_tar_bz2", args: []string{"-Z", "archive.tar.bz2"}},
		{name: "119_expand_archive_tar", args: []string{"-Z", "archive.tar"}},
		{name: "119b_archive_image_html", args: []string{"--html", "-Z", "images.zip"}},
		{name: "119c_archive_html_converted", args: []string{"-Z", "-D", "docs.zip"}},
	})
}

func TestGoldenDocumentURLMediaType(t *testing.T) {
	dir := setupDocumentsFixture(t)
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer server.Close()

	runTestGolden(t, dir, []goldenTestCase{
		{name: "139e_url_html_by_media_type", args: []string{"-D", server.URL + "/suffixless"}},
		{name: "139f_url_html_no_extract_flag", args: []string{server.URL + "/suffixless"}},
	}, server.URL)
}

func TestGoldenDocuments(t *testing.T) {
	dir := setupDocumentsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "120_docs_pdf_extracted", args: []string{"-D", "sample.pdf"}},
		{name: "121_docs_docx_extracted", args: []string{"-D", "sample.docx"}},
		{name: "122_docs_xlsx_extracted", args: []string{"-D", "sample.xlsx"}},
		{name: "123_docs_no_extract_flag", args: []string{"sample.pdf", "sample.docx", "sample.xlsx"}},
		{name: "124_docs_odt_expanded", args: []string{"-Z", "sample.odt"}},
		{name: "125_docs_odt_binary", args: []string{"sample.odt"}},
		{name: "126_docs_pptx_extracted", args: []string{"-D", "sample.pptx"}},
		{name: "127_docs_ods_expanded", args: []string{"-Z", "sample.ods"}},
		{name: "128_docs_odp_expanded", args: []string{"-Z", "sample.odp"}},
		{name: "129_docs_ods_binary", args: []string{"sample.ods"}},
		{name: "130_docs_odp_binary", args: []string{"sample.odp"}},
		{name: "131_docs_docx_numbered", args: []string{"-D", "-l", "sample.docx"}},
		{name: "132_docs_xlsx_head", args: []string{"-D", "--head", "2", "sample.xlsx"}},
		{name: "133_docs_pptx_lines", args: []string{"-D", "--lines", "3", "sample.pptx"}},
		{name: "134_docs_pdf_xml", args: []string{"--xml", "-D", "sample.pdf"}},
		{name: "135_docs_html_markdown", args: []string{"-D", "sample.html"}},
		{name: "136_docs_html_no_extract_flag", args: []string{"sample.html"}},
		{name: "137_docs_html_xml", args: []string{"--xml", "-D", "sample.html"}},
		{name: "138_docs_html_head", args: []string{"-D", "--head", "6", "sample.html"}},
		{name: "139_docs_htm_extension", args: []string{"-D", "sample.htm"}},
		{name: "139b_docs_xhtml_extension", args: []string{"-D", "sample.xhtml"}},
		{name: "139c_docs_html_line_numbers", args: []string{"-D", "-l", "sample.html"}},
		{name: "139d_docs_html_into_html_output", args: []string{"--html", "-D", "sample.html"}},
	})
}

func TestGoldenMedia(t *testing.T) {
	dir := setupMediaFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "140_media_mp4_video_and_audio", args: []string{"-M", "sample.mp4"}},
		{name: "141_media_mp4_video_only", args: []string{"-M", "silent.mp4"}},
		{name: "142_media_mov", args: []string{"-M", "sample.mov"}},
		{name: "143_media_m4a_audio_only", args: []string{"-M", "sample.m4a"}},
		{name: "144_media_wav", args: []string{"-M", "sample.wav"}},
		{name: "145_media_flac", args: []string{"-M", "sample.flac"}},
		{name: "146_media_truncated_header", args: []string{"-M", "truncated.mp4"}},
		{name: "147_media_no_extract_flag", args: []string{"sample.mp4", "sample.wav"}},
		{name: "148_media_directory", args: []string{"-M", "-e", "*.zip", "."}},
		{name: "149_media_xml", args: []string{"--xml", "-M", "sample.wav"}},
		{name: "150_media_line_numbers", args: []string{"-M", "-l", "sample.mp4"}},
		{name: "151_media_head", args: []string{"-M", "--head", "2", "sample.mp4"}},
		{name: "152_media_in_archive", args: []string{"-Z", "-M", "media.zip"}},
		{name: "153_media_with_documents", args: []string{"-D", "-M", "sample.wav"}},
		{name: "154_media_stats", args: []string{"-M", "--stats", "sample.mp4", "notes.txt"}},
		{name: "155_media_mkv", args: []string{"-M", "sample.mkv"}},
		{name: "156_media_webm", args: []string{"-M", "sample.webm"}},
		{name: "157_media_mp3", args: []string{"-M", "sample.mp3"}},
		{name: "158_media_png", args: []string{"-M", "sample.png"}},
		{name: "159_media_jpeg", args: []string{"-M", "sample.jpg"}},
		{name: "160_media_gif_animated", args: []string{"-M", "sample.gif"}},
		{name: "161_media_webp", args: []string{"-M", "sample.webp"}},
		{name: "162_media_bmp", args: []string{"-M", "sample.bmp"}},
		{name: "163_media_tiff", args: []string{"-M", "sample.tiff"}},
		{name: "164_media_avif", args: []string{"-M", "sample.avif"}},
		{name: "165_media_images_no_flag", args: []string{"sample.png", "sample.gif"}},
		{name: "166_media_image_html_embeds", args: []string{"--html", "-M", "sample.png"}},
		{name: "167_media_suffixless_sniffed", args: []string{"-M", "suffixless"}},
		{name: "168_media_suffixless_no_flag", args: []string{"suffixless"}},
		{name: "169_media_wrong_suffix", args: []string{"-M", "mislabelled.jpg"}},
		{name: "170_media_suffixless_not_media", args: []string{"-M", "notes"}},
	})
}

// Media over HTTP arrives as a stream rather than a file, with a declared
// length (FileServer) or without one (chunked), and must describe itself the
// same way a local file does.
func TestGoldenMediaURL(t *testing.T) {
	dir := setupMediaFixture(t)
	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, chunked := strings.CutPrefix(r.URL.Path, "/chunked")
		if !chunked {
			files.ServeHTTP(w, r)
			return
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// Flushing before the body leaves the length unknown, so it is chunked.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		w.Write(data)
	}))
	defer server.Close()

	u := server.URL
	runTestGolden(t, dir, []goldenTestCase{
		{name: "170j_media_url_mp4", args: []string{"-M", u + "/sample.mp4"}},
		{name: "170k_media_url_mkv_webm", args: []string{"-M", u + "/sample.mkv", u + "/sample.webm"}},
		{name: "170l_media_url_audio", args: []string{"-M", u + "/sample.mp3", u + "/sample.wav", u + "/sample.flac", u + "/sample.m4a"}},
		{name: "170m_media_url_images", args: []string{"-M", u + "/sample.gif", u + "/sample.webp", u + "/sample.tiff", u + "/sample.jpg", u + "/sample.png", u + "/sample.bmp", u + "/sample.avif"}},
		{name: "170n_media_url_no_flag", args: []string{u + "/sample.mp4", u + "/notes.txt"}},
		{name: "170o_media_url_stats", args: []string{"-M", "--stats", u + "/sample.mp4", u + "/notes.txt"}},
		{name: "170p_media_url_chunked", args: []string{"-M", u + "/chunked/sample.mp4", u + "/chunked/sample.mkv", u + "/chunked/notes.txt"}},
		{name: "170q_media_url_truncated_suffixless", args: []string{"-M", u + "/truncated.mp4", u + "/suffixless", u + "/mislabelled.jpg"}},
		{name: "170r_media_url_html_embeds", args: []string{"--html", "-M", u + "/sample.png"}},
		{name: "170s_url_compact", args: []string{"-n0", u + "/notes.txt", u + "/chunked/notes.txt", u + "/sample.mp4"}},
	}, u)
}

// Content decides over the name: text is never media, whatever it is called or
// however it starts.
func TestGoldenMediaLookalikes(t *testing.T) {
	dir := setupMediaLookalikeFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "170b_media_weak_magic_text", args: []string{"-M", "bm25"}},
		{name: "170c_media_lfs_pointer_png", args: []string{"-M", "x.png"}},
		{name: "170d_media_lfs_pointer_png_html", args: []string{"--html", "x.png"}},
		{name: "170e_media_tif_extension", args: []string{"-M", "x.tif"}},
		{name: "170f_media_suffixless_image", args: []string{"-M", "avatar"}},
		{name: "170g_media_suffixless_image_html", args: []string{"--html", "avatar"}},
		{name: "170h_media_weak_magic_binary", args: []string{"-M", "counts.dat", "data.bin"}},
		{name: "170i_media_weak_magic_binary_html", args: []string{"--html", "counts.dat", "data.bin"}},
	})
}
