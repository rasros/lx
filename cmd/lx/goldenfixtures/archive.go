package goldenfixtures

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mholt/archives"
)

func SetupArchiveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	createTestZip(filepath.Join(dir, "archive.zip"), [][2]string{
		{"hello.txt", "Hello from archive!\n"},
		{"nested/world.go", "package nested\n"},
		{".hidden_in_zip", "hidden inside zip\n"},
	})
	createTestTarGz(filepath.Join(dir, "archive.tar.gz"), [][2]string{
		{"hello.txt", "Hello from tar!\n"},
		{"nested/world.go", "package nested\n"},
	})
	createTestTarBz2(filepath.Join(dir, "archive.tar.bz2"), [][2]string{
		{"hello.txt", "Hello from tar bz2!\n"},
		{"nested/world.go", "package nested\n"},
	})
	createTestZip(filepath.Join(dir, "docs.zip"), [][2]string{
		{"guide.html", "<h1>Guide</h1><p>Inside an archive.</p>"},
		{"notes.txt", "plain\n"},
	})
	createTestGz(filepath.Join(dir, "notes.txt.gz"), "compressed on its own\nsecond line\n")
	createTestZip(filepath.Join(dir, "images.zip"), [][2]string{
		{"logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\x0D"},
		{"notes.txt", "beside the image\n"},
	})
	createTestTar(filepath.Join(dir, "archive.tar"), [][2]string{
		{"hello.txt", "Hello from tar plain!\n"},
		{"nested/world.go", "package nested\n"},
	})

	return dir
}

func SetupDocumentsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, dir, "sample.html", htmlFixture, 0644)
	writeFile(t, dir, "sample.htm", "<h1>Legacy</h1><p>htm extension</p>", 0644)
	writeFile(t, dir, "sample.xhtml", "<h1>Strict</h1><p>xhtml extension</p>", 0644)
	writeFile(t, dir, "suffixless", htmlFixture, 0644)

	fixtures := []string{
		"sample.pdf", "sample.docx", "sample.xlsx",
		"sample.pptx",
		"sample.odt", "sample.ods", "sample.odp",
	}
	for _, name := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", "documents", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	return dir
}

func createTestTarGz(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, file := range files {
		body := []byte(file[1])
		hdr := &tar.Header{Name: file[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
		if _, err := tw.Write(body); err != nil {
			panic("createTestTarGz: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
	if err := gw.Close(); err != nil {
		panic("createTestTarGz: " + err.Error())
	}
}

func createTestTarBz2(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestTarBz2: " + err.Error())
	}
	defer f.Close()

	bw, err := archives.Bz2{}.OpenWriter(f)
	if err != nil {
		panic("createTestTarBz2: " + err.Error())
	}

	tw := tar.NewWriter(bw)
	for _, file := range files {
		body := []byte(file[1])
		hdr := &tar.Header{Name: file[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("createTestTarBz2: " + err.Error())
		}
		if _, err := tw.Write(body); err != nil {
			panic("createTestTarBz2: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("createTestTarBz2: " + err.Error())
	}
	if err := bw.Close(); err != nil {
		panic("createTestTarBz2: " + err.Error())
	}
}

func createTestTar(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestTar: " + err.Error())
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	for _, file := range files {
		body := []byte(file[1])
		hdr := &tar.Header{Name: file[0], Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("createTestTar: " + err.Error())
		}
		if _, err := tw.Write(body); err != nil {
			panic("createTestTar: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("createTestTar: " + err.Error())
	}
}

func createTestGz(path, content string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	// Real gzip records the original name, so the fixture does too.
	zw.Name = strings.TrimSuffix(filepath.Base(path), ".gz")
	zw.Write([]byte(content))
	zw.Close()
}

// SetupRepoArchiveFixture writes repo.zip, laid out as a repository host
// serves a repository: every file below one "repo-main/" directory, with
// ignore files at the root and below it. Beside it are single compressed
// files: notes.txt.gz, the hidden .bashrc.gz and plain.tar.gz, which holds
// no tar archive. nowrap.zip has no one directory its files are all below,
// and malformed.zip, wrapped in "repo-main/", has entries no checkout could
// hold beside names a checkout keeps as they are. configs/ holds the
// configs of SetupArchiveOriginFixture.
func SetupRepoArchiveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	createTestZip(filepath.Join(dir, "repo.zip"), [][2]string{
		{"repo-main/", ""},
		{"repo-main/.gitignore", "build/\n*.log\n"},
		{"repo-main/.lxignore", "LICENSE\n"},
		{"repo-main/LICENSE", "MIT\n"},
		{"repo-main/README.md", "# repo\n"},
		{"repo-main/app.log", "log\n"},
		{"repo-main/build/out.txt", "built\n"},
		{"repo-main/docs/.ignore", "drafts/\n"},
		{"repo-main/docs/drafts/wip.md", "wip\n"},
		{"repo-main/docs/guide.md", "guide\n"},
		{"repo-main/docs/deep/.gitignore", "*.tmp\n"},
		{"repo-main/docs/deep/more.md", "more\n"},
		{"repo-main/docs/deep/scratch.tmp", "tmp\n"},
		{"repo-main/internal/cli/app.go", "package cli\n"},
		{"repo-main/internal/cli/app_test.go", "package cli\n"},
		{"repo-main/internal/other/o.go", "package other\n"},
		{"repo-main/.github/ci.yml", "on: push\n"},
	})
	createTestZip(filepath.Join(dir, "nowrap.zip"), [][2]string{
		{"a/x.txt", "a x\n"},
		{"b/x.txt", "b x\n"},
		{"b/y.txt", "b y\n"},
		{"TOP.txt", "top\n"},
	})
	createTestZip(filepath.Join(dir, "malformed.zip"), [][2]string{
		{"repo-main/", ""},
		{"repo-main/../up.txt", "up\n"},
		{"repo-main/.", ""},
		{"repo-main/..", ""},
		{"repo-main/ok.txt", "ok\n"},
		{`repo-main/back\slash.txt`, "backslash\n"},
		{`repo-main/x\y.txt`, "backslashed x y\n"},
		{"repo-main/x/y.txt", "x y\n"},
		{"repo-main/x/y.txt", "second x y\n"},
		{"repo-main/f", "a file\n"},
		{"repo-main/f/below.txt", "below a file\n"},
	})
	writeOriginConfigs(t, dir)
	createTestGz(filepath.Join(dir, "notes.txt.gz"), "compressed on its own\n")
	createTestGz(filepath.Join(dir, ".bashrc.gz"), "export EDITOR=vi\n")
	createTestGz(filepath.Join(dir, "plain.tar.gz"), "not a tar archive\n")
	return dir
}

// SetupArchiveDepthFixture writes the same files twice, below arch/ as
// archives at three depths and below dirs/ as directories of the same
// names, so the globs can be shown to treat them alike.
func SetupArchiveDepthFixture(t *testing.T) (arch, dirs string) {
	t.Helper()
	root := t.TempDir()
	arch, dirs = filepath.Join(root, "arch"), filepath.Join(root, "dirs")
	entries := [][2]string{
		{"src/a.go", "package src\n"},
		{"src/b.txt", "b\n"},
		{"deep/src/a.go", "package deep\n"},
		{"notes.txt", "n\n"},
	}
	for _, a := range []string{"top.zip", "d1/mid.zip", "d1/d2/low.zip"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(arch, a)), 0755); err != nil {
			t.Fatal(err)
		}
		createTestZip(filepath.Join(arch, a), entries)
		for _, e := range entries {
			writeFile(t, dirs, filepath.Join(a, e[0]), e[1], 0644)
		}
	}
	return arch, dirs
}

func createTestZip(path string, files [][2]string) {
	f, err := os.Create(path)
	if err != nil {
		panic("createTestZip: " + err.Error())
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, file := range files {
		fw, err := w.Create(file[0])
		if err != nil {
			panic("createTestZip: " + err.Error())
		}
		if _, err := fw.Write([]byte(file[1])); err != nil {
			panic("createTestZip: " + err.Error())
		}
	}
}

const htmlFixture = `<!doctype html>
<html><head><title>API Reference</title>
<style>body { color: red }</style>
<script>var tracked = 1;</script>
</head>
<body>
<nav><a href="/">Home</a></nav>
<h1>Authentication</h1>
<p>All endpoints require a <code>Bearer</code> token. See <a href="/auth">auth docs</a>.</p>
<hr>
<h2>Example</h2>
<pre><code class="language-go">func main() {
	fmt.Println("hi")
}</code></pre>
<ul><li>First</li><li>Second<ul><li>Nested</li></ul></li></ul>
<blockquote><p>Rate limits apply.</p></blockquote>
<table><thead><tr><th>Field</th><th>Type</th></tr></thead>
<tbody><tr><td>id</td><td>int</td></tr></tbody></table>
<p><img src="data:image/png;base64,AAAABBBBCCCC" alt="inline blob"></p>
<p><img src="/logo.png" alt="Logo"></p>
<footer>Copyright 2026</footer>
</body></html>
`
