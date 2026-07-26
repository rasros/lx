package cli

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSystemContextFields(t *testing.T) {
	at := time.Date(2026, 7, 19, 14, 55, 0, 0, time.UTC)
	fields, _ := systemContext([]string{"-D", "-t", "./project"}, at)

	if got := fields["os"]; got != runtime.GOOS {
		t.Errorf("os = %q, want %q", got, runtime.GOOS)
	}
	if got := fields["arch"]; got != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", got, runtime.GOARCH)
	}
	if got := fields["time"]; got != "2026-07-19T14:55:00Z" {
		t.Errorf("time = %q, want the injected timestamp", got)
	}
	if got := fields["version"]; got != Version {
		t.Errorf("version = %q, want %q", got, Version)
	}
	if got := fields["command"]; got != "lx -D -t ./project" {
		t.Errorf("command = %q", got)
	}
}

func TestSystemContextBodyOrderAndLabels(t *testing.T) {
	body := systemContextBody(map[string]string{
		"command": "lx .",
		"version": "v1.2.3",
		"host":    "workstation",
		"time":    "2026-07-19T14:55:00Z",
		"arch":    "amd64",
		"os":      "linux",
	})

	want := strings.Join([]string{
		"OS: linux",
		"Arch: amd64",
		"Time: 2026-07-19T14:55:00Z",
		"Host: workstation",
		"Tool: lx v1.2.3",
		"Command: lx .",
	}, "\n") + "\n"

	if body != want {
		t.Errorf("body =\n%q\nwant\n%q", body, want)
	}
}

func TestSystemContextBodySkipsMissingFields(t *testing.T) {
	body := systemContextBody(map[string]string{"os": "linux", "host": ""})

	if strings.Contains(body, "Host:") {
		t.Errorf("body includes an empty Host line:\n%q", body)
	}
	if !strings.Contains(body, "OS: linux") {
		t.Errorf("body missing OS line:\n%q", body)
	}
}

func TestSystemContextBodyAppendsUnknownFieldsAlphabetically(t *testing.T) {
	body := systemContextBody(map[string]string{
		"os":     "linux",
		"zone":   "utc",
		"branch": "main",
	})

	want := "OS: linux\nbranch: main\nzone: utc\n"
	if body != want {
		t.Errorf("body =\n%q\nwant\n%q", body, want)
	}
}

func TestFormatCommandQuotesAmbiguousArguments(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{}, "lx"},
		{[]string{"-i", "*.go", "."}, "lx -i *.go ."},
		{[]string{"-s", "My Section"}, `lx -s "My Section"`},
		{[]string{"-p", "it's here"}, `lx -p "it's here"`},
		{[]string{""}, `lx ""`},
	}

	for _, c := range cases {
		if got := formatCommand(c.args); got != c.want {
			t.Errorf("formatCommand(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}
