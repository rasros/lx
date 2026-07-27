package sources

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestIsHTTPURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "http", raw: "http://example.com/a.txt", want: true},
		{name: "https", raw: "https://example.com/a.txt?x=1", want: true},
		{name: "uppercase_scheme", raw: "HTTP://example.com", want: true},
		{name: "unsupported_scheme", raw: "ftp://example.com/a.txt", want: false},
		{name: "missing_scheme", raw: "example.com/a.txt", want: false},
		{name: "missing_host", raw: "https://", want: false},
		{name: "local_path", raw: "./a.txt", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsHTTPURL(tc.raw)
			if got != tc.want {
				t.Fatalf("IsHTTPURL(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsHTTPArchiveURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "zip", raw: "https://example.com/a.zip", want: true},
		{name: "zip_with_query", raw: "https://example.com/a.zip?download=1", want: true},
		{name: "txt", raw: "https://example.com/a.txt", want: false},
		{name: "unsupported_scheme", raw: "ftp://example.com/a.zip", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsHTTPArchiveURL(tc.raw)
			if got != tc.want {
				t.Fatalf("IsHTTPArchiveURL(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNewURLInputFile_OpenSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte("remote body"))
	}))
	defer server.Close()

	f, err := NewURLInputFile(server.URL + "/notes.txt")
	if err != nil {
		t.Fatalf("NewURLInputFile error: %v", err)
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(body) != "remote body" {
		t.Fatalf("body = %q, want %q", string(body), "remote body")
	}
}

func TestNewURLInputFile_OpenStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	f, err := NewURLInputFile(server.URL + "/missing.txt")
	if err != nil {
		t.Fatalf("NewURLInputFile error: %v", err)
	}

	rc, err := f.Open()
	if rc != nil {
		rc.Close()
		t.Fatalf("Open returned non-nil reader on error")
	}
	if err == nil {
		t.Fatalf("Open expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("Open error = %q, want status error", err.Error())
	}
}

func TestNewURLInputFile_RejectsUnsupportedScheme(t *testing.T) {
	_, err := NewURLInputFile("ftp://example.com/archive.zip")
	if err == nil {
		t.Fatalf("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported URI scheme") {
		t.Fatalf("error = %q, want unsupported scheme", err.Error())
	}
}

func TestDownloadURLToTempFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	defer server.Close()

	path, cleanup, err := DownloadURLToTempFile(context.Background(), server.URL+"/main.zip")
	if err != nil {
		t.Fatalf("DownloadURLToTempFile error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "zip-bytes" {
		t.Fatalf("downloaded content = %q, want %q", string(data), "zip-bytes")
	}
}

func TestDownloadURLToTempFile_StatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusBadGateway)
	}))
	defer server.Close()

	_, _, err := DownloadURLToTempFile(context.Background(), server.URL+"/main.zip")
	if err == nil {
		t.Fatalf("expected status error")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("error = %q, want status error", err.Error())
	}
}

func TestDownloadURLToTempFile_DownloadedArchiveDetectable(t *testing.T) {
	zipBody := makeArchiveBytes(t, map[string]string{"a.txt": "A"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBody)
	}))
	defer server.Close()

	path, cleanup, err := DownloadURLToTempFile(context.Background(), server.URL+"/main.zip")
	if err != nil {
		t.Fatalf("DownloadURLToTempFile error: %v", err)
	}
	defer cleanup()

	if !IsArchivePath(path) {
		t.Fatalf("downloaded temp path %q is not recognized as archive", path)
	}
}

func makeArchiveBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatalf("WriteString(%q): %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestNewURLInputFileRecordsMediaType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<h1>Hi</h1>"))
	}))
	defer srv.Close()

	f, err := NewURLInputFile(srv.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	if IsHTMLInput(f) {
		t.Error("IsHTMLInput = true before Open, when no media type is known yet")
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()

	if got := *f.mediaType; got != "text/html; charset=utf-8" {
		t.Errorf("recorded media type = %q", got)
	}
	if !IsHTMLInput(f) {
		t.Error("IsHTMLInput = false for a text/html response")
	}
}

func TestIsHTMLInputIgnoresOtherMediaTypes(t *testing.T) {
	for _, ct := range []string{"text/plain", "application/octet-stream", "", "not a media type"} {
		f := InputFile{Path: "https://example.com/docs", mediaType: &ct}
		if IsHTMLInput(f) {
			t.Errorf("IsHTMLInput = true for Content-Type %q", ct)
		}
	}
	for _, ct := range []string{"text/html", "text/html; charset=utf-8", "application/xhtml+xml"} {
		f := InputFile{Path: "https://example.com/docs", mediaType: &ct}
		if !IsHTMLInput(f) {
			t.Errorf("IsHTMLInput = false for Content-Type %q", ct)
		}
	}
}

func TestIsHTMLInputFallsBackToSuffix(t *testing.T) {
	if !IsHTMLInput(InputFile{Path: "page.html"}) {
		t.Error("IsHTMLInput = false for page.html")
	}
	if IsHTMLInput(InputFile{Path: "main.go"}) {
		t.Error("IsHTMLInput = true for main.go")
	}
}
