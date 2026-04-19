package goldenfixtures

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholt/archives"
)

func SetupArchiveFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
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
	createTestTar(filepath.Join(dir, "archive.tar"), [][2]string{
		{"hello.txt", "Hello from tar plain!\n"},
		{"nested/world.go", "package nested\n"},
	})

	return dir
}

func SetupDocumentsFixture(t *testing.T) string {
	t.Helper()
	setupMockConfig(t)
	dir := t.TempDir()

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
