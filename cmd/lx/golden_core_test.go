package main

import "testing"

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

func TestGoldenSymlinks(t *testing.T) {
	dir := setupSymlinksFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
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
	})
}

func TestGoldenConfig(t *testing.T) {
	dir := setupConfigFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "090_config_hidden", args: []string{"-H", "."}},
		{name: "091_config_follow", args: []string{"--follow", "links"}},
		{name: "092_config_combined", args: []string{"-H", "--follow", "links"}},
		{name: "093_force_file_flag", args: []string{"-f", ".gitignore"}},
		{name: "094_force_file_flag_excludes", args: []string{"-e", "*.go", "-f", "main.go"}},
		{name: "095_config_custom_sections", args: []string{"-y", "configs/custom_sections.yaml", "-s", "Head", "main.go"}},
		{name: "096_direct_file_parent_ignore", args: []string{"direct_ignore_test/test.ignored", "direct_ignore_test/test.kept"}},
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
		{name: "113_expand_archive_hidden", args: []string{"-Z", "-H", "archive.zip"}},
		{name: "114_expand_archive_line_numbers", args: []string{"-Z", "-l", "archive.zip"}},
		{name: "115_expand_archive_tar_gz", args: []string{"-Z", "archive.tar.gz"}},
		{name: "116_expand_archive_xml", args: []string{"--xml", "-Z", "archive.zip"}},
		{name: "117_expand_archive_hidden_filter_go", args: []string{"-Z", "-H", "-i", "*.go", "archive.zip"}},
		{name: "118_expand_archive_tar_bz2", args: []string{"-Z", "archive.tar.bz2"}},
		{name: "119_expand_archive_tar", args: []string{"-Z", "archive.tar"}},
	})
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
	})
}
