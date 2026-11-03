package templatex

import (
	"errors"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/internal"
)

// profile is an output format as data: the templates it defaults to and how
// the output is laid out around them. Config keys override each property.
type profile struct {
	FileContent   string
	FileError     string
	FileBinary    string
	FileCompact   string
	FileHeader    string
	FileImage     string
	Section       string
	Prompt        string
	Tree          string
	Meta          string
	OutputHeader  string
	OutputFooter  string
	SectionHeader string
	SectionFooter string

	// Separators puts a blank line between items, or a newline between
	// compact ones.
	Separators bool
	// EmbedImages reads images into FileContext.DataURI instead of
	// describing them.
	EmbedImages bool
}

// imageAsBinary renders an image as the binary file template does, for the
// formats that do not embed images.
const imageAsBinary = `{{ template "file_binary" . }}`

// profiles are the built-in output formats by name.
var profiles = map[string]profile{
	"markdown": {
		FileContent:  defaultMarkdownContent,
		FileError:    defaultMarkdownError,
		FileBinary:   defaultMarkdownBinary,
		FileCompact:  defaultMarkdownCompact,
		FileHeader:   defaultMarkdownFileHeader,
		FileImage:    imageAsBinary,
		Section:      defaultMarkdownSection,
		Prompt:       defaultPrompt,
		Tree:         defaultMarkdownTree,
		Meta:         defaultMarkdownMeta,
		OutputFooter: defaultMarkdownOutputFooter,
		Separators:   true,
	},
	"xml": {
		FileContent:   defaultXMLContent,
		FileError:     defaultXMLError,
		FileBinary:    defaultXMLBinary,
		FileCompact:   defaultXMLCompact,
		FileImage:     imageAsBinary,
		Section:       defaultXMLSection,
		SectionHeader: defaultXMLSectionHeader,
		SectionFooter: defaultXMLSectionFooter,
		Prompt:        defaultXMLPrompt,
		Tree:          defaultXMLTree,
		Meta:          defaultXMLMeta,
		OutputFooter:  defaultXMLOutputFooter,
		Separators:    true,
	},
	"html": {
		FileContent:   defaultHTMLContent,
		FileError:     defaultHTMLError,
		FileBinary:    defaultHTMLBinary,
		FileCompact:   defaultHTMLCompact,
		FileImage:     defaultHTMLImage,
		Section:       defaultHTMLSection,
		SectionHeader: defaultHTMLSectionHeader,
		SectionFooter: defaultHTMLSectionFooter,
		Prompt:        defaultHTMLPrompt,
		Tree:          defaultHTMLTree,
		Meta:          defaultHTMLMeta,
		OutputHeader:  defaultHTMLOutputHeader,
		OutputFooter:  defaultHTMLOutputFooter,
		EmbedImages:   true,
	},
	"bare": {
		FileContent: defaultBareContent,
		FileError:   defaultBareError,
		FileBinary:  defaultBareBinary,
		FileCompact: defaultBareCompact,
		FileHeader:  defaultBareFileHeader,
		FileImage:   imageAsBinary,
		Section:     defaultBareSection,
		Prompt:      defaultBarePrompt,
		Tree:        defaultBareTree,
		Meta:        defaultBareMeta,
	},
}

// profileFor returns the profile of format, Markdown's when it is empty, or
// an error naming the formats there are.
func profileFor(format string) (profile, error) {
	if format == "" {
		format = "markdown"
	}
	if p, ok := profiles[format]; ok {
		return p, nil
	}
	return profile{}, fmt.Errorf("unknown output format %q: want markdown, xml, html or bare", format)
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
const defaultXMLSectionHeader = `{{ if .IsImplicit }}<content>{{ else }}<section>{{ end }}
`
const defaultXMLSection = `  <section_name>{{ .Body }}</section_name>`
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
  {{ if not .Global.SizeUnknown }}Size: {{ .Global.TotalSize | humanize }} &bull; {{ end }}
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
{{- if and (gt .Size 0) (not .IsError) }}
 <small>
{{- if .IsCompactView -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows - compact)
{{- else -}}
   ({{ if .IsEstimate }}~{{ end }}{{ .TotalRows }} rows)
{{- end -}}
 </small>
{{- end }}
</header>
<pre><code{{ if .Language }} class="language-{{ .Language }}"{{ end }}>
{{ .Content | escape | endNewline }}</code></pre>
</article>
`

const defaultHTMLImage = `<article id="file-{{ .FileIndex }}">
<header>
 <a href="#file-{{ .FileIndex }}" aria-label="Link to this file"><strong>{{ .Path }}</strong></a>
</header>
<img src="{{ .DataURI }}" alt="{{ .Path }}" />
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

const defaultHTMLSectionHeader = `{{ if not .IsImplicit }}<section id="section-{{ .Index }}">{{ end }}`
const defaultHTMLSection = `<h2><a href="#section-{{ .Index }}" style="text-decoration:none; color:inherit;">{{ .Body | endNewline }}</a></h2>`
const defaultHTMLSectionFooter = `{{ if not .IsImplicit }}</section>{{ end }}`
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

// text is a template value as text, without copying a string.
func text(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// Funcs returns the functions every template can call, with dataURI
// resolving relative paths against dir, or the process's working directory
// when it is empty.
func Funcs(dir string) template.FuncMap {
	return templateFuncs(&core.TemplateEngine{Dir: dir})
}

// templateFuncs returns the functions templates of engine can call.
func templateFuncs(engine *core.TemplateEngine) template.FuncMap {
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
			str := text(s)
			if str != "" && !strings.HasSuffix(str, "\n") {
				return str + "\n"
			}
			return str
		},
		// dataURI reads a file on disk, so an archive entry's empty AbsPath
		// fails; use .DataURI for those.
		"dataURI": func(path string) (string, error) {
			if path == "" {
				return "", errors.New("dataURI: no file on disk to read; an archive entry has no AbsPath, and .DataURI holds an image's content")
			}
			abs := path
			if !filepath.IsAbs(abs) && engine.Dir != "" {
				abs = filepath.Join(engine.Dir, abs)
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return "", nil
			}
			return internal.DataURI(path, data), nil
		},
		"escape": func(s interface{}) string {
			return html.EscapeString(text(s))
		},
		"commafy": func(n interface{}) string {
			v, ok := asInt64(n)
			if !ok {
				return fmt.Sprintf("%v", n)
			}
			return Commafy(v)
		},
		"plural": func(n interface{}, singular, plural string) string {
			if v, ok := asInt64(n); ok && v == 1 {
				return singular
			}
			return plural
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

// Commafy writes v with a comma between each group of three digits, as the
// commafy template function does.
func Commafy(v int64) string {
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
