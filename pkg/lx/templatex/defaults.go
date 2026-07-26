package templatex

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

type formatDefaults struct {
	FileContent   string
	FileError     string
	FileBinary    string
	FileCompact   string
	FileHeader    string
	Section       string
	Prompt        string
	Tree          string
	Meta          string
	OutputHeader  string
	OutputFooter  string
	SectionHeader string
	SectionFooter string
}

func getFormatDefaults(fmtType string) formatDefaults {
	switch fmtType {
	case "xml":
		return formatDefaults{
			FileContent:   defaultXMLContent,
			FileError:     defaultXMLError,
			FileBinary:    defaultXMLBinary,
			FileCompact:   defaultXMLCompact,
			Section:       defaultXMLSection,
			SectionHeader: defaultXMLSectionHeader,
			SectionFooter: defaultXMLSectionFooter,
			Prompt:        defaultXMLPrompt,
			Tree:          defaultXMLTree,
			Meta:          defaultXMLMeta,
			OutputFooter:  defaultXMLOutputFooter,
		}
	case "html":
		return formatDefaults{
			FileContent:   defaultHTMLContent,
			FileError:     defaultHTMLError,
			FileBinary:    defaultHTMLBinary,
			FileCompact:   defaultHTMLCompact,
			Section:       defaultHTMLSection,
			SectionHeader: defaultHTMLSectionHeader,
			SectionFooter: defaultHTMLSectionFooter,
			Prompt:        defaultHTMLPrompt,
			Tree:          defaultHTMLTree,
			Meta:          defaultHTMLMeta,
			OutputHeader:  defaultHTMLOutputHeader,
			OutputFooter:  defaultHTMLOutputFooter,
		}
	case "bare":
		return formatDefaults{
			FileContent: defaultBareContent,
			FileError:   defaultBareError,
			FileBinary:  defaultBareBinary,
			FileCompact: defaultBareCompact,
			FileHeader:  defaultBareFileHeader,
			Section:     defaultBareSection,
			Prompt:      defaultBarePrompt,
			Tree:        defaultBareTree,
			Meta:        defaultBareMeta,
		}
	default: // markdown
		return formatDefaults{
			FileContent:  defaultMarkdownContent,
			FileError:    defaultMarkdownError,
			FileBinary:   defaultMarkdownBinary,
			FileCompact:  defaultMarkdownCompact,
			FileHeader:   defaultMarkdownFileHeader,
			Section:      defaultMarkdownSection,
			Prompt:       defaultPrompt,
			Tree:         defaultMarkdownTree,
			Meta:         defaultMarkdownMeta,
			OutputFooter: defaultMarkdownOutputFooter,
		}
	}
}

const defaultMarkdownFileHeader = `{{ if gt .Section.TotalFiles 1 }}[{{ .SectionFileIndex }}/{{ .Section.TotalFiles }}] {{ end }}{{ .Path }}`
const defaultMarkdownContent = `{{ template "file_header" . }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows{{ if .SkeletonMode }}, {{ .SkeletonMode }}{{ end }})
---
` + "```{{ .Language }}" + `
{{ .Content | endNewline -}}` + "```"

const defaultMarkdownError = `{{ template "file_header" . }} - error: {{ .ReadError }}`
const defaultMarkdownBinary = `{{ template "file_header" . }} - binary file skipped ({{ .Size | humanize }})`
const defaultMarkdownCompact = `{{ template "file_header" . }} ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows{{ if .SkeletonMode }}, {{ .SkeletonMode }}{{ end }})`
const defaultMarkdownSection = `## {{ .Body | endNewline }}---`
const defaultPrompt = `{{ .Body | endNewline }}`
const defaultMarkdownTree = "```\n" + `{{ .Body | endNewline -}}` + "```"
const defaultMarkdownMeta = "```\n" + `{{ .Body | endNewline -}}` + "```"
const defaultMarkdownOutputFooter = "\n\n"

const defaultXMLContent = `  <document index="{{ .FileIndex }}"{{ if .Language }} language="{{ .Language }}"{{ end }} rows="{{ .TotalRows }}"{{ if .SkeletonMode }} skeleton="{{ .SkeletonMode }}"{{ end }}>
    <source>{{ .Path }}</source>
    <document_content>
{{ .Content | endNewline }}    </document_content>
  </document>`
const defaultXMLError = `  <document index="{{ .FileIndex }}">
    <source>{{ .Path }}</source>
    <error>{{ .ReadError }}</error>
  </document>`
const defaultXMLBinary = `  <document index="{{ .FileIndex }}">
    <source>{{ .Path }}</source>
    <error>Binary file ({{ .Size | humanize }})</error>
  </document>`
const defaultXMLCompact = `  <document index="{{ .FileIndex }}">
    <source>{{ .Path }}</source>
    <description>Compact view</description>
  </document>`
const defaultXMLSectionHeader = `{{ if .IsImplicit }}<content>
{{ end }}`
const defaultXMLSection = `{{- if not .IsImplicit }}<section>
  <section_name>{{ .Body }}</section_name>{{ end }}`
const defaultXMLSectionFooter = `
{{ if .IsImplicit }}</content>{{ else }}</section>{{ end }}`
const defaultXMLPrompt = `  <instruction>` + "\n" +
	`{{ .Body | endNewline }}` +
	`  </instruction>`
const defaultXMLTree = `  <tree>
{{ .Body | endNewline }}  </tree>`
const defaultXMLMeta = `  <system_context>
{{ .Body | endNewline }}  </system_context>`
const defaultXMLOutputFooter = "\n\n"

const defaultHTMLOutputHeader = `<!DOCTYPE html>
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
.error-state { color: var(--pico-color-red-500); font-weight: bold; }
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
const defaultHTMLOutputFooter = `</main>
<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js"></script>
<script>hljs.highlightAll();</script>
</body>
</html>
`
const defaultHTMLContent = `<article id="file-{{ .FileIndex }}">
<header>
 <a href="#file-{{ .FileIndex }}" aria-label="Link to this file"><strong>{{ .Path }}</strong></a>
{{- if and (gt .Size 0) (not .IsImage) (not .IsError) }}
 <small>
{{- if .IsCompactView -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows - compact)
{{- else -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)
{{- end -}}
 </small>
{{- end }}
</header>
{{- if .IsImage }}
<img src="{{ .AbsPath | dataURI }}" alt="{{ .Path }}" />
{{- else }}
<pre><code{{ if .Language }} class="language-{{ .Language }}"{{ end }}>
{{ .Content | escape | endNewline }}</code></pre>
{{- end }}
</article>
`

const defaultHTMLError = `<article id="file-{{ .FileIndex }}">
<header>
 <strong class="file-anchor">{{ .Path }}</strong>
</header>
<p class="error-state">Error: {{ .ReadError }}</p>
</article>
`
const defaultHTMLBinary = `<article id="file-{{ .FileIndex }}">
<header>
 <strong class="file-anchor">{{ .Path }}</strong>
</header>
<em>Binary file ({{ .Size | humanize }})</em>
</article>
`
const defaultHTMLCompact = `<article id="file-{{ .FileIndex }}">
<header>
 <strong class="file-anchor">{{ .Path }}</strong>
 <small>({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows - compact)</small>
</header>
<em>Compact view detected</em>
</article>
`

const defaultHTMLSectionHeader = ``
const defaultHTMLSection = `<section id="section-{{ .Index }}"><h2><a href="#section-{{ .Index }}" style="text-decoration:none; color:inherit;">{{ .Body | endNewline }}</a></h2>`
const defaultHTMLSectionFooter = `</section>`
const defaultHTMLPrompt = `<blockquote>{{ .Body | endNewline }}</blockquote>`
const defaultHTMLTree = `<pre class="file-tree">{{ .Body | escape | endNewline }}</pre>`
const defaultHTMLMeta = `<pre class="system-context">{{ .Body | escape | endNewline }}</pre>`

const defaultBareFileHeader = ``
const defaultBareContent = `{{ .Content | endNewline -}}`
const defaultBareError = ``
const defaultBareBinary = ``
const defaultBareCompact = ``
const defaultBareSection = ``
const defaultBarePrompt = ``
const defaultBareTree = ``
const defaultBareMeta = ``

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
)

// 128k matches GPT-4o context, 200k matches Claude Opus.
const (
	tokenWarnThreshold   int64 = 128_000
	tokenDangerThreshold int64 = 200_000
)

const defaultStatsTemplate = `{{ accent .ColorEnabled "▎" }} ` +
	`{{ commafy .Global.TotalFiles }} {{ plural .Global.TotalFiles "file" "files" }} {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.TotalRows }} {{ plural .Global.TotalRows "row" "rows" }} {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.TotalSections }} {{ plural .Global.TotalSections "section" "sections" }} {{ dim .ColorEnabled "·" }} ` +
	`{{ humanize .Global.TotalSize }}` + "\n" +
	`  {{ tokenLabel .Global.TokenEstimate .ColorEnabled }}` + "\n"

// The report is a diagnostic summary rather than bundle content, so it has one
// default across formats, like the stats summary.
const defaultReportTemplate = `{{ if .Files }}` +
	"| File | Original | Rendered | Tokens |\n" +
	"|------|----------|----------|--------|\n" +
	`{{ range .Files }}| {{ .Path }} | {{ humanize .OriginalSize }} | {{ humanize .RenderedSize }} | {{ commafy .Tokens }} |` + "\n" +
	`{{ end }}` + "\n" +
	`{{ end }}` +
	`{{ if .Top }}Largest: {{ range $i, $f := .Top }}{{ if $i }}, {{ end }}{{ $f.Path }}{{ end }}` + "\n" +
	`{{ end }}` +
	`{{ commafy .Global.TotalFiles }} processed {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.SkippedBySize }} skipped (size) {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.BinaryFiles }} binary {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.FailedFiles }} failed` + "\n" +
	`  {{ tokenLabel .Global.TokenEstimate .ColorEnabled }}` + "\n"

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
			idx := int(e)
			if idx >= len(sizes) {
				idx = len(sizes) - 1
			}
			suffix := sizes[idx]
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
		"commafy": func(n interface{}) string {
			v, ok := asInt64(n)
			if !ok {
				return fmt.Sprintf("%v", n)
			}
			return commafy(v)
		},
		"plural": func(n interface{}, singular, plural string) string {
			if v, ok := asInt64(n); ok && v == 1 {
				return singular
			}
			return plural
		},
		"accent": func(enabled bool, s string) string {
			if !enabled {
				return s
			}
			return ansiCyan + s + ansiReset
		},
		"dim": func(enabled bool, s string) string {
			if !enabled {
				return s
			}
			return ansiDim + s + ansiReset
		},
		"tokenLabel": func(n int64, enabled bool) string {
			label := fmt.Sprintf("~%s tokens", commafy(n))
			if !enabled {
				return label
			}
			color := ansiGreen
			switch {
			case n >= tokenDangerThreshold:
				color = ansiRed
			case n >= tokenWarnThreshold:
				color = ansiYellow
			}
			return color + ansiBold + label + ansiReset
		},
	}
}

func asInt64(n interface{}) (int64, bool) {
	switch x := n.(type) {
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	}
	return 0, false
}

func commafy(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d", v)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		b.WriteByte(',')
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// TemplateFuncs exposes the default template function map.
func TemplateFuncs() template.FuncMap { return templateFuncs() }
