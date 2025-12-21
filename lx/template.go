package lx

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

// DefaultTemplate uses explicit newlines and avoids raw string line-breaks
// that would introduce unwanted spacing.
const DefaultTemplate = `{{ if .IsBinary }}` +
	`{{ if gt .TotalFiles 1 }}[{{ .FileIndex }}/{{ .TotalFiles }}] {{ end }}{{ .Path }} - binary file skipped ({{ .Size | humanize }})` + "\n\n" +
	`{{ else }}` +
	`{{ if gt .TotalFiles 1 }}[{{ .FileIndex }}/{{ .TotalFiles }}] {{ end }}{{ .Path }} ({{ .TotalRows }} rows)
---
` + "```{{ .Language }}" + `
{{ .Content | endNewline -}}` + "```\n\n" + `{{ end }}`

// DefaultSectionTemplate is used by the -s flag.
const DefaultSectionTemplate = `## {{ .Body | endNewline }}` + "---\n\n"

// DefaultPromptTemplate is used by the -p flag.
const DefaultPromptTemplate = `{{ .Body | endNewline }}` + "\n"

func IsBinaryData(data []byte) bool {
	limit := 1024
	if len(data) < limit {
		limit = len(data)
	}
	if bytes.IndexByte(data[:limit], 0) != -1 {
		return true
	}
	return !utf8.Valid(data)
}

type FileContext struct {
	Path       string
	Size       int64
	ModTime    time.Time
	TotalRows  int
	Language   string
	Content    string
	IsBinary   bool
	FileIndex  int
	TotalFiles int
}

func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"date": func(layout string, t time.Time) string {
			return t.Format(layout)
		},
		"humanize": func(s int64) string {
			sizes := []string{"B", "kB", "MB", "GB", "TB"}
			if s < 1000 {
				return fmt.Sprintf("%d B", s)
			}
			e := math.Floor(math.Log(float64(s)) / math.Log(1000))
			suffix := sizes[int(e)]
			val := float64(s) / math.Pow(1000, e)
			return fmt.Sprintf("%.1f %s", val, suffix)
		},
		"endNewline": func(s string) string {
			if s != "" && !strings.HasSuffix(s, "\n") {
				return s + "\n"
			}
			return s
		},
	}
}
