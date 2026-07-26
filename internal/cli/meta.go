package cli

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

// systemContext returns the fields for a --system-context block and the
// pre-rendered body built from them.
func systemContext(args []string, now time.Time) (map[string]string, string) {
	fields := map[string]string{
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"time":    now.Format(time.RFC3339),
		"version": Version,
		"command": formatCommand(args),
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		fields["host"] = host
	}
	return fields, systemContextBody(fields)
}

// Ordered so the block reads consistently; unknown keys from a caller-supplied
// map are appended alphabetically rather than dropped.
var systemContextOrder = []struct {
	key    string
	label  string
	prefix string
}{
	{"os", "OS", ""},
	{"arch", "Arch", ""},
	{"time", "Time", ""},
	{"host", "Host", ""},
	{"version", "Tool", "lx "},
	{"command", "Command", ""},
}

func systemContextBody(fields map[string]string) string {
	var sb strings.Builder
	seen := make(map[string]bool, len(systemContextOrder))

	for _, f := range systemContextOrder {
		seen[f.key] = true
		if v := fields[f.key]; v != "" {
			fmt.Fprintf(&sb, "%s: %s%s\n", f.label, f.prefix, v)
		}
	}

	extra := make([]string, 0, len(fields))
	for k := range fields {
		if !seen[k] && fields[k] != "" {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		fmt.Fprintf(&sb, "%s: %s\n", k, fields[k])
	}

	return sb.String()
}

// formatCommand renders the invocation for the block, quoting only arguments
// that would otherwise be ambiguous to read back.
func formatCommand(args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, "lx")
	for _, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			parts = append(parts, fmt.Sprintf("%q", a))
			continue
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}
