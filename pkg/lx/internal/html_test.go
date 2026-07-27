package internal

import (
	"strings"
	"testing"
)

func toMarkdown(t *testing.T, in string) string {
	t.Helper()
	got, err := HTMLToMarkdown(strings.NewReader(in))
	if err != nil {
		t.Fatalf("HTMLToMarkdown failed: %v", err)
	}
	return strings.TrimSpace(string(got))
}

func TestHTMLToMarkdownStructure(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"heading levels", "<h1>One</h1><h3>Three</h3>", "# One\n\n### Three"},
		{"horizontal rule", "<p>a</p><hr><p>b</p>", "a\n\n---\n\nb"},
		{"paragraphs", "<p>first</p><p>second</p>", "first\n\nsecond"},
		{"link", `<p>see <a href="/docs">docs</a></p>`, "see [docs](/docs)"},
		{"link without href", "<p>see <a>docs</a></p>", "see docs"},
		{"inline code", "<p>use <code>make()</code></p>", "use `make()`"},
		{"image", `<p><img src="/a.png" alt="Logo"></p>`, "![Logo](/a.png)"},
		{"blockquote", "<blockquote><p>quoted</p></blockquote>", "> quoted"},
		{"unordered list", "<ul><li>a</li><li>b</li></ul>", "- a\n- b"},
		{"ordered list start", `<ol start="3"><li>c</li><li>d</li></ol>`, "3. c\n4. d"},
		{"nested list", "<ul><li>a<ul><li>b</li></ul></li></ul>", "- a\n  - b"},
		{"title is not content", "<html><head><title>T</title></head><body><p>x</p></body></html>", "x"},
		{"whitespace collapses", "<p>a\n\n   b\tc</p>", "a b c"},
		{"emphasis is dropped", "<p>a <strong>b</strong> <em>c</em></p>", "a b c"},
		{"block level image", `<img src="/a.png" alt="A">`, "![A](/a.png)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := toMarkdown(t, c.in); got != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

func TestHTMLToMarkdownDropsChrome(t *testing.T) {
	in := `<html><head><title>T</title><style>b{color:red}</style>
	<script>var x=1</script></head><body>
	<nav>Home</nav><header>Top</header><aside>Ads</aside>
	<p>kept</p><footer>Bottom</footer><noscript>NS</noscript></body></html>`

	got := toMarkdown(t, in)
	for _, unwanted := range []string{"color:red", "var x", "Home", "Top", "Ads", "Bottom", "NS"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("output kept %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "kept") {
		t.Errorf("output lost the body text:\n%s", got)
	}
}

func TestHTMLToMarkdownPreservesCodeBlocks(t *testing.T) {
	in := "<pre><code class=\"language-go\">func main() {\n\tif x {\n\t\treturn\n\t}\n}</code></pre>"
	want := "```go\nfunc main() {\n\tif x {\n\t\treturn\n\t}\n}\n```"

	if got := toMarkdown(t, in); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestHTMLToMarkdownCodeFenceWithoutLanguage(t *testing.T) {
	if got := toMarkdown(t, "<pre>plain\n  text</pre>"); got != "```\nplain\n  text\n```" {
		t.Errorf("got:\n%q", got)
	}
}

func TestHTMLToMarkdownStripsDataURIs(t *testing.T) {
	got := toMarkdown(t, `<p><img src="data:image/png;base64,AAAABBBB" alt="blob"></p>`)
	if strings.Contains(got, "base64") || strings.Contains(got, "data:") {
		t.Errorf("data URI survived: %s", got)
	}
	if got != "![blob]" {
		t.Errorf("got %q, want %q", got, "![blob]")
	}
}

func TestHTMLToMarkdownDataURILinkKeepsText(t *testing.T) {
	got := toMarkdown(t, `<p><a href="data:text/plain,hi">click</a></p>`)
	if got != "click" {
		t.Errorf("got %q, want %q", got, "click")
	}
}

func TestHTMLToMarkdownTable(t *testing.T) {
	in := `<table><thead><tr><th>Field</th><th>Type</th></tr></thead>
	<tbody><tr><td>id</td><td>int</td></tr></tbody></table>`
	want := "| Field | Type |\n|---|---|\n| id | int |"

	if got := toMarkdown(t, in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHTMLToMarkdownTableEscapesPipes(t *testing.T) {
	got := toMarkdown(t, "<table><tr><th>a</th></tr><tr><td>x | y</td></tr></table>")
	if !strings.Contains(got, `x \| y`) {
		t.Errorf("pipe not escaped:\n%s", got)
	}
}

func TestHTMLToMarkdownTablePadsShortRows(t *testing.T) {
	got := toMarkdown(t, "<table><tr><th>a</th><th>b</th></tr><tr><td>x</td></tr></table>")
	want := "| a | b |\n|---|---|\n| x |  |"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHTMLToMarkdownHandlesMalformedInput(t *testing.T) {
	got := toMarkdown(t, "<p>unclosed <b>bold <div>and a div")
	if !strings.Contains(got, "unclosed") || !strings.Contains(got, "and a div") {
		t.Errorf("lost content from malformed input:\n%s", got)
	}
}

func TestHTMLToMarkdownEmptyDocument(t *testing.T) {
	if got := toMarkdown(t, ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHTMLToMarkdownDefinitionListsAndBreaks(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"definition list", "<dl><dt>Term</dt><dd>Meaning</dd></dl>", "Term\n\nMeaning"},
		{"br splits a line", "<p>a<br>b</p>", "a b"},
		{"deep nesting", "<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li></ul>", "- a\n  - b\n    - c"},
		{"pre without code", "<pre>  spaced  </pre>", "```\n  spaced  \n```"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := toMarkdown(t, c.in); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Page metadata is not content: nothing from head, including meta tags, appears.
func TestHTMLToMarkdownIgnoresHeadMetadata(t *testing.T) {
	in := `<html><head>
	<title>Page Title</title>
	<meta name="description" content="A description">
	<meta property="og:title" content="Social Title">
	<link rel="canonical" href="/canonical">
	</head><body><h1>Real Heading</h1><p>Body.</p></body></html>`

	got := toMarkdown(t, in)
	for _, unwanted := range []string{"Page Title", "A description", "Social Title", "canonical"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("output includes head metadata %q:\n%s", unwanted, got)
		}
	}
	if got != "# Real Heading\n\nBody." {
		t.Errorf("got:\n%s", got)
	}
}

// A stray meta in the body must not leak either.
func TestHTMLToMarkdownIgnoresMetaInBody(t *testing.T) {
	got := toMarkdown(t, `<body><p>a</p><meta name="x" content="leaked"><p>b</p></body>`)
	if strings.Contains(got, "leaked") {
		t.Errorf("meta content leaked:\n%s", got)
	}
}
