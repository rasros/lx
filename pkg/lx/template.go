package lx

import (
	"fmt"
	"math"
	"strings"
	"text/template"
	"time"
)

const DefaultTemplate = `{{ if eq .Size 0 }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} - empty file` + "\n" +
	`{{ else if .IsBinary }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} - binary file skipped ({{ .Size | humanize }})` + "\n" +
	`{{ else if .IsCompactView }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)` + "\n" +
	`{{ else }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)
---
` + "```{{ .Language }}" + `
{{ .Content | endNewline -}}` + "```\n\n" + `{{ end }}`

const DefaultSectionTemplate = `## {{ .Body | endNewline }}` + "---\n\n"
const DefaultPromptTemplate = `{{ .Body | endNewline }}` + "\n"

const DefaultXMLTemplate = `<document index="{{ .FileIndex }}"{{ if .Language }} language="{{ .Language }}"{{ end }} rows="{{ .TotalRows }}">` + "\n" +
	`  <source>{{ .Path }}</source>` + "\n" +
	`  {{- if .IsBinary }}` + "\n" +
	`  <error>Binary file ({{ .Size | humanize }})</error>` + "\n" +
	`  {{- else if .IsCompactView }}` + "\n" +
	`  <description>Compact view</description>` + "\n" +
	`  {{- else }}` + "\n" +
	`  <document_content>` + "\n" +
	`{{ .Content | endNewline }}  </document_content>` + "\n" +
	`  {{- end }}` + "\n" +
	`</document>` + "\n\n"

const DefaultXMLSectionTemplate = `<section_header>` + "\n" +
	`{{ .Body | endNewline -}}` +
	`</section_header>` + "\n\n"

const DefaultXMLPromptTemplate = `<instruction>` + "\n" +
	`{{ .Body | endNewline -}}` +
	`</instruction>` + "\n\n"

const DefaultDebugTemplate = `Files: {{ .Global.TotalFiles }}` + "\n" +
	`Size: {{ .Global.TotalSize | humanize }}` + "\n" +
	`Est. Tokens: {{ .Global.TokenEstimate }}` + "\n"

type GlobalContext struct {
	TotalFiles    int
	TotalSize     int64
	TokenEstimate int64
	TotalSections int
	RootPath      string
	AbsRootPath   string
	Args          map[string]string
	Config        Config
}

type FileContext struct {
	Path           string
	AbsPath        string
	Size           int64
	ModTime        time.Time
	TotalRows      int
	TokenEstimate  int64
	IsEstimate     bool
	Language       string
	Content        interface{}
	IsBinary       bool
	IsCompactView  bool
	FileIndex      int
	CurrentSection int
	Global         GlobalContext
}

type SectionContext struct {
	Body    string
	Section int
	Global  GlobalContext
}

type PromptContext struct {
	Body    string
	Section int
	Global  GlobalContext
}

type DebugContext struct {
	Global GlobalContext
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
		"endNewline": func(s interface{}) string {
			if str, ok := s.(string); ok {
				if str != "" && !strings.HasSuffix(str, "\n") {
					return str + "\n"
				}
				return str
			}
			return fmt.Sprintf("%v", s)
		},
	}
}
