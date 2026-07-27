package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// IsHTTPURL reports whether raw is a valid http/https URI.
func IsHTTPURL(raw string) bool {
	_, err := parseHTTPURL(raw)
	return err == nil
}

// IsHTTPArchiveURL reports whether raw is an http/https URI whose path is an archive.
func IsHTTPArchiveURL(raw string) bool {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return false
	}
	return IsArchivePath(u.Path)
}

// NewURLInputFile creates an InputFile that fetches content by HTTP GET.
func NewURLInputFile(raw string) (InputFile, error) {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return InputFile{}, err
	}

	uri := u.String()
	mediaType := new(string)
	return InputFile{
		Path:      uri,
		AbsPath:   uri,
		Size:      -1,
		mediaType: mediaType,
		Open: func() (io.ReadCloser, error) {
			req, err := http.NewRequest(http.MethodGet, uri, nil)
			if err != nil {
				return nil, err
			}
			resp, err := defaultHTTPClient.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				resp.Body.Close()
				return nil, fmt.Errorf("GET %q: unexpected status %s", uri, resp.Status)
			}
			*mediaType = resp.Header.Get("Content-Type")
			return resp.Body, nil
		},
	}, nil
}

// DownloadURLToTempFile downloads a URL with HTTP GET to a temporary file.
// Caller must invoke cleanup after processing completes.
func DownloadURLToTempFile(ctx context.Context, raw string) (path string, cleanup func(), err error) {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return "", nil, err
	}
	uri := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("GET %q: unexpected status %s", uri, resp.Status)
	}

	ext := filepath.Ext(u.Path)
	tmp, err := os.CreateTemp("", "lx-url-*"+ext)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		return "", nil, err
	}
	ok = true

	return tmpPath, func() { _ = os.Remove(tmpPath) }, nil
}

func parseHTTPURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("URI is empty")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URI %q: %w", raw, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported URI scheme: %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URI %q has no host", raw)
	}
	return u, nil
}
