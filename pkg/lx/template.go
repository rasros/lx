package lx

import (
	"encoding/base64"
	"fmt"
	"html"
	"math"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Markdown Templates: Dynamic spacing via .Separator.
const defaultTemplate = `{{ .Separator }}{{ if eq .Size 0 }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} - empty file` +
	`{{ else if .IsBinary }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} - binary file skipped ({{ .Size | humanize }})` +
	`{{ else if .IsCompactView }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)` +
	`{{ else }}` +
	`{{ if gt .Global.TotalFiles 1 }}[{{ .FileIndex }}/{{ .Global.TotalFiles }}] {{ end }}{{ .Path }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)
---
` + "```{{ .Language }}" + `
{{ .Content | endNewline -}}` + "```" + `{{ end }}`

const defaultSectionTemplate = `{{ .Separator }}## {{ .Body | endNewline }}---`
const defaultPromptTemplate = `{{ .Separator }}{{ .Body | endNewline }}`

const defaultHeaderTemplate = ""

// defaultFooterTemplate provides the final double newline for MD/XML streams.
// HTML overrides this in CompileTemplates.
const defaultFooterTemplate = "\n\n"

// XML Templates: Now use .Separator for dynamic spacing too.
const defaultXMLTemplate = `{{ .Separator }}<document index="{{ .FileIndex }}"{{ if .Language }} language="{{ .Language }}"{{ end }} rows="{{ .TotalRows }}">` + "\n" +
	`  <source>{{ .Path }}</source>` + "\n" +
	`  {{- if .IsBinary }}` + "\n" +
	`  <error>Binary file ({{ .Size | humanize }})</error>` + "\n" +
	`  {{- else if .IsCompactView }}` + "\n" +
	`  <description>Compact view</description>` + "\n" +
	`  {{- else }}` + "\n" +
	`  <document_content>` + "\n" +
	`{{ .Content | endNewline }}  </document_content>` + "\n" +
	`  {{- end }}` + "\n" +
	`</document>`

const defaultXMLSectionTemplate = `{{ .Separator }}<section_header>` + "\n" +
	`{{ .Body | endNewline -}}` +
	`</section_header>`

const defaultXMLPromptTemplate = `{{ .Separator }}<instruction>` + "\n" +
	`{{ .Body | endNewline -}}` +
	`</instruction>`

// HTML Templates: Do NOT use .Separator. They handle structure internally.
const defaultHTMLHeaderTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>lx Output</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.classless.min.css">
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css" media="(prefers-color-scheme: light)">
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css" media="(prefers-color-scheme: dark)">
<style>
.hljs { background: transparent !important; }
img { max-width: 100%; height: auto; }
pre { background-color: transparent; border: none; }
article > header {
  position: sticky;
  top: 0;
  z-index: 10;
  background-color: var(--pico-card-background-color); 
  border-bottom: 1px solid var(--pico-muted-border-color);
}
.file-anchor {
  text-decoration: none;
  color: var(--pico-muted-color);
  margin-right: 0.5rem;
  font-weight: bold;
  border: none;
}
.file-anchor:hover {
  color: var(--pico-primary);
  text-decoration: underline;
}
</style>
</head>
<body>
<header>
<hgroup>
<h1>lx output</h1>
<p>
  Files: {{ .Global.TotalFiles }} &bull; 
  Size: {{ .Global.TotalSize | humanize }} &bull; 
  Path: {{ .Global.WorkDir }}
</p>
</hgroup>
</header>
<main>
`
const defaultHTMLFooterTemplate = `</main>
<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js"></script>
<script>hljs.highlightAll();</script>
</body>
</html>
`
const defaultHTMLTemplate = `<article id="file-{{ .FileIndex }}">
<header>
 <a href="#file-{{ .FileIndex }}" aria-label="Link to this file"><strong>{{ .Path }}</strong></a>
{{- if and (gt .Size 0) (not .IsImage) }}
 <small>
 {{- if .IsCompactView -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows - compact)
 {{- else -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)
 {{- end -}}
 </small>
{{- end }}
</header>
{{- if eq .Size 0 }}
<em>Empty file</em>
{{- else if .IsImage }}
<img src="{{ .AbsPath | dataURI }}" alt="{{ .Path }}" />
{{- else if .IsBinary }}
<em>Binary file ({{ .Size | humanize }})</em>
{{- else if .IsCompactView }}
<em>Compact view detected</em>
{{- else }}
<pre><code{{ if .Language }} class="language-{{ .Language }}"{{ end }}>
{{ .Content | escape | endNewline }}</code></pre>
{{- end }}
</article>
`

const defaultHTMLSectionTemplate = `<section id="section-{{ .Section }}">` +
	`<h2><a href="#section-{{ .Section }}" style="text-decoration:none; color:inherit;">{{ .Body | endNewline }}</a></h2>` +
	`</section>`

const defaultHTMLPromptTemplate = `<blockquote>{{ .Body | endNewline }}</blockquote>`

const defaultStatsTemplate = `Files: {{ .Global.TotalFiles }}` + "\n" +
	`Size: {{ .Global.TotalSize | humanize }}` + "\n" +
	`Est. Tokens: {{ .Global.TokenEstimate }}` + "\n"

func templateFuncs() template.FuncMap {
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
		"dataURI": func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			ext := strings.ToLower(filepath.Ext(path))
			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				switch ext {
				case ".svg":
					mimeType = "image/svg+xml"
				case ".jpg", ".jpeg":
					mimeType = "image/jpeg"
				case ".png":
					mimeType = "image/png"
				case ".gif":
					mimeType = "image/gif"
				case ".webp":
					mimeType = "image/webp"
				default:
					mimeType = "application/octet-stream"
				}
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			return fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
		},
		"escape": func(s interface{}) string {
			if str, ok := s.(string); ok {
				return html.EscapeString(str)
			}
			return fmt.Sprintf("%v", s)
		},
	}
}
