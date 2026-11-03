package sources

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// A body that stalls mid-download stops reading once ctx is done.
func TestNewURLInputFile_CancelStopsStalledBody(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first bytes"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	f, err := NewURLInputFile(server.URL + "/slow.txt")
	if err != nil {
		t.Fatalf("NewURLInputFile error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc, err := f.Open(ctx)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer rc.Close()

	time.AfterFunc(20*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(rc)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadAll error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reading a stalled body did not stop when cancelled")
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
