package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx"
	"github.com/rasros/lx/pkg/lx/sources"
)

func TestRun_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "hello.txt")
	os.WriteFile(f1, []byte("Hello World"), 0644)

	tests := []struct {
		name        string
		args        []string
		wantContain []string
	}{
		{
			name:        "version flag",
			args:        []string{"-V"},
			wantContain: []string{"lx version"},
		},
		{
			name:        "render file",
			args:        []string{f1},
			wantContain: []string{"hello.txt", "Hello World"},
		},
		{
			name:        "section and prompt",
			args:        []string{"-s", "MyHeader", "-p", "Instructions", f1},
			wantContain: []string{"## MyHeader", "Instructions", "Hello World"},
		},
		{
			name:        "new flags check",
			args:        []string{"--no-links", "--no-ignore", f1},
			wantContain: []string{"hello.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCapture(t, tt.args)

			if err != nil && !strings.Contains(tt.name, "error") {
				t.Fatalf("Run() unexpected error: %v", err)
			}

			for _, s := range tt.wantContain {
				if !strings.Contains(out, s) {
					t.Errorf("Output missing %q. Got:\n%s", s, out)
				}
			}
		})
	}
}

func TestRun_StickyFlags(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "long.txt")
	os.WriteFile(f1, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	out, err := runCapture(t, []string{"-n", "2", f1})

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out, "line3") {
		t.Errorf("Output should have been sliced. Got:\n%s", out)
	}
}

func TestApplyOutputFormatOverrides_LastFlagWins(t *testing.T) {
	cfg := lx.NewConfig()
	cfg.OutputFormat = "xml"

	parsed := &ParsedArgs{
		Ops: []Op{
			{Action: "html", Type: CmdGlobal},
			{Action: "md", Type: CmdGlobal},
			{Action: "bare", Type: CmdGlobal},
		},
	}

	applyOutputFormatOverrides(cfg, parsed)

	if cfg.OutputFormat != "bare" {
		t.Fatalf("cfg.OutputFormat = %q, want %q", cfg.OutputFormat, "bare")
	}
}

func TestRun_HTTPURLInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte("hello from remote"))
	}))
	defer server.Close()

	url := server.URL + "/sample.txt"
	out, err := runCapture(t, []string{url})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(out, url) {
		t.Fatalf("output missing URL %q. Got:\n%s", url, out)
	}
	if !strings.Contains(out, "hello from remote") {
		t.Fatalf("output missing remote body. Got:\n%s", out)
	}
}

func TestRun_HTTPURLArchiveInput_Expand(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("repo-main/README.md")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("hello archive")); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBuf.Bytes())
	}))
	defer server.Close()

	url := server.URL + "/main.zip"
	out, err := runCapture(t, []string{"-Z", url})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(out, url+"/repo-main/README.md") {
		t.Fatalf("output missing expanded archive entry. Got:\n%s", out)
	}
	if !strings.Contains(out, "hello archive") {
		t.Fatalf("output missing expanded archive content. Got:\n%s", out)
	}
}

func TestRun_ShortRepoURL_AutoExpand(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("repo-main/README.md")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("hello archive")); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBuf.Bytes())
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	restore := sources.SetRepoHostForTest(serverURL.Host, func(owner, repo, ref string) string {
		if ref == "" {
			ref = "HEAD"
		}
		return server.URL + "/" + owner + "/" + repo + "/archive/" + ref + ".zip"
	})
	defer restore()

	short := serverURL.Host + "/owner/repo"
	out, err := runCapture(t, []string{short})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if want := "/owner/repo/archive/HEAD.zip"; gotPath != want {
		t.Fatalf("server got path %q, want %q", gotPath, want)
	}
	if !strings.Contains(out, "/repo-main/README.md") {
		t.Fatalf("output missing expanded archive entry. Got:\n%s", out)
	}
	if !strings.Contains(out, "hello archive") {
		t.Fatalf("output missing expanded archive content. Got:\n%s", out)
	}
}

// testEnv returns an Env that writes to buffers and runs in a temporary
// working directory with an empty user config.
func testEnv(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	home := t.TempDir()
	vars := map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "NO_COLOR": "1"}
	return Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		LookupEnv: func(k string) (string, bool) {
			v, ok := vars[k]
			return v, ok
		},
		Cwd: t.TempDir(),
		Clipboard: func(string) error {
			t.Error("unexpected clipboard write")
			return nil
		},
	}, &stdout, &stderr
}

// runCapture runs lx with args and returns what it wrote to stdout.
func runCapture(t *testing.T, args []string) (string, error) {
	t.Helper()
	env, stdout, _ := testEnv(t)
	err := Run(context.Background(), args, env)
	return stdout.String(), err
}

func TestDetermineOutput_StdoutOverridesCopyDefault(t *testing.T) {
	env, _, _ := testEnv(t)
	out := determineOutput(map[string]string{"stdout": "true"}, "copy", env, slog.Default())
	if out.w != env.Stdout {
		t.Fatalf("out = %T, want env.Stdout", out.w)
	}
	if out.clip != nil {
		t.Fatalf("clip should be nil when stdout is forced")
	}
	if out.debug != env.Stderr {
		t.Fatalf("debug should stay on stderr when stdout is forced")
	}
}

func TestDetermineOutput_OutputWinsOverStdoutAndCopy(t *testing.T) {
	env, _, _ := testEnv(t)
	out := determineOutput(map[string]string{"output": "out.txt", "stdout": "true", "copy": "true"}, "copy", env, slog.Default())
	if want := filepath.Join(env.Cwd, "out.txt"); out.path != want || out.name != "out.txt" {
		t.Fatalf("output file = %q (%q), want %q", out.path, out.name, want)
	}
	if out.clip != nil {
		t.Fatalf("clip should be nil when output file is used")
	}
	if out.debug != env.Stdout {
		t.Fatalf("debug should be stdout when output file is used")
	}
}
