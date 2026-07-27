package internal

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Inline elements join the surrounding paragraph instead of starting one.
var htmlInline = map[atom.Atom]bool{
	atom.A: true, atom.Img: true, atom.Code: true, atom.Span: true,
	atom.Strong: true, atom.B: true, atom.Em: true, atom.I: true,
	atom.Small: true, atom.Abbr: true, atom.Cite: true, atom.Kbd: true,
	atom.Samp: true, atom.Var: true, atom.Sub: true, atom.Sup: true,
	atom.Time: true, atom.Q: true, atom.S: true, atom.Del: true,
	atom.Ins: true, atom.Mark: true, atom.U: true, atom.Label: true,
}

var htmlDropped = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Noscript: true,
	atom.Nav:      true,
	atom.Header:   true,
	atom.Footer:   true,
	atom.Aside:    true,
	atom.Template: true,
	atom.Svg:      true,
	atom.Head:     true,
}

// HTMLToMarkdown converts a document to structural Markdown: headings, rules,
// lists, blockquotes, fenced code, tables, links and images. Only the body is
// rendered; the title is page metadata rather than content.
//
// Inline emphasis is deliberately dropped: keeping it would mean escaping every
// literal * and _ in body text, for a signal a model barely uses.
func HTMLToMarkdown(r io.Reader) ([]byte, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	w := &markdownWriter{}
	root := htmlBody(doc)
	if root == nil {
		root = doc
	}
	w.walk(root)

	return []byte(w.result()), nil
}

type markdownWriter struct {
	blocks  []string
	pending strings.Builder
}

func (w *markdownWriter) block(s string) {
	w.flush()
	if s = strings.Trim(s, "\n"); s != "" {
		w.blocks = append(w.blocks, s)
	}
}

func (w *markdownWriter) flush() {
	if text := collapseSpace(w.pending.String()); text != "" {
		w.blocks = append(w.blocks, text)
	}
	w.pending.Reset()
}

func (w *markdownWriter) result() string {
	w.flush()
	return strings.Join(w.blocks, "\n\n") + "\n"
}

func (w *markdownWriter) walk(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			w.pending.WriteString(c.Data)

		case html.ElementNode:
			if htmlDropped[c.DataAtom] {
				continue
			}
			switch c.DataAtom {
			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				level, _ := strconv.Atoi(strings.TrimPrefix(c.Data, "h"))
				if level < 1 || level > 6 {
					level = 1
				}
				w.block(strings.Repeat("#", level) + " " + inlineText(c))
			case atom.Hr:
				w.block("---")
			case atom.P:
				w.block(inlineText(c))
			case atom.Pre:
				w.block(codeFence(c))
			case atom.Ul, atom.Ol:
				w.block(renderList(c, 0))
			case atom.Blockquote:
				w.block(prefixLines(subdocument(c), "> "))
			case atom.Table:
				w.block(renderTable(c))
			case atom.Dt:
				w.block(inlineText(c))
			case atom.Dd:
				w.block(inlineText(c))
			case atom.Br:
				w.pending.WriteString("\n")
			default:
				if htmlInline[c.DataAtom] {
					writeInlineNode(&w.pending, c)
					continue
				}
				w.walk(c)
			}
		}
	}
}

// subdocument renders a container separately, for constructs that prefix every
// line of their content.
func subdocument(n *html.Node) string {
	sub := &markdownWriter{}
	sub.walk(n)
	return strings.TrimSuffix(sub.result(), "\n")
}

func inlineText(n *html.Node) string {
	var sb strings.Builder
	writeInline(&sb, n)
	return collapseSpace(sb.String())
}

func writeInline(sb *strings.Builder, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeInlineNode(sb, c)
	}
}

func writeInlineNode(sb *strings.Builder, c *html.Node) {
	switch c.Type {
	case html.TextNode:
		sb.WriteString(c.Data)
	case html.ElementNode:
		if htmlDropped[c.DataAtom] {
			return
		}
		switch c.DataAtom {
		case atom.A:
			text := inlineText(c)
			if href := usableURL(attr(c, "href")); href != "" {
				sb.WriteString("[" + text + "](" + href + ")")
				return
			}
			sb.WriteString(text)
		case atom.Img:
			alt := attr(c, "alt")
			if src := usableURL(attr(c, "src")); src != "" {
				sb.WriteString("![" + alt + "](" + src + ")")
				return
			}
			if alt != "" {
				sb.WriteString("![" + alt + "]")
			}
		case atom.Code:
			if text := collapseSpace(rawText(c)); text != "" {
				sb.WriteString("`" + text + "`")
			}
		case atom.Br:
			sb.WriteString(" ")
		default:
			writeInline(sb, c)
		}
	}
}

// usableURL rejects data: URIs, where the base64 payloads that bloat a bundle
// live.
func usableURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		return ""
	}
	return trimmed
}

func renderList(list *html.Node, depth int) string {
	indent := strings.Repeat("  ", depth)
	ordered := list.DataAtom == atom.Ol
	number := 1
	if start, err := strconv.Atoi(attr(list, "start")); err == nil && ordered {
		number = start
	}

	var lines []string
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.DataAtom != atom.Li {
			continue
		}

		marker := "- "
		if ordered {
			marker = strconv.Itoa(number) + ". "
			number++
		}

		var nested []string
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.DataAtom == atom.Ul || c.DataAtom == atom.Ol) {
				nested = append(nested, renderList(c, depth+1))
			}
		}

		lines = append(lines, indent+marker+listItemText(li))
		lines = append(lines, nested...)
	}
	return strings.Join(lines, "\n")
}

// listItemText is an item's own text, excluding any nested list.
func listItemText(li *html.Node) string {
	var sb strings.Builder
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.DataAtom == atom.Ul || c.DataAtom == atom.Ol) {
			continue
		}
		writeInlineNode(&sb, c)
	}
	return collapseSpace(sb.String())
}

// codeFence keeps pre content verbatim, labelled from a highlight.js or prism
// class when present.
func codeFence(pre *html.Node) string {
	lang := codeLanguage(pre)
	if inner := firstElement(pre, atom.Code); inner != nil {
		if lang == "" {
			lang = codeLanguage(inner)
		}
	}

	body := strings.Trim(rawText(pre), "\n")
	return "```" + lang + "\n" + body + "\n```"
}

func codeLanguage(n *html.Node) string {
	for _, class := range strings.Fields(attr(n, "class")) {
		for _, prefix := range []string{"language-", "lang-"} {
			if strings.HasPrefix(class, prefix) {
				return strings.TrimPrefix(class, prefix)
			}
		}
	}
	return ""
}

// renderTable collapses each cell to one line; rowspan and colspan are ignored,
// since neither survives the format.
func renderTable(table *html.Node) string {
	var rows [][]string
	collectRows(table, &rows)
	if len(rows) == 0 {
		return ""
	}

	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}

	pad := func(r []string) []string {
		for len(r) < width {
			r = append(r, "")
		}
		return r
	}

	var lines []string
	lines = append(lines, "| "+strings.Join(pad(rows[0]), " | ")+" |")
	lines = append(lines, "|"+strings.Repeat("---|", width))
	for _, r := range rows[1:] {
		lines = append(lines, "| "+strings.Join(pad(r), " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func collectRows(n *html.Node, rows *[][]string) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if c.DataAtom == atom.Tr {
			var cells []string
			for cell := c.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type == html.ElementNode && (cell.DataAtom == atom.Th || cell.DataAtom == atom.Td) {
					cells = append(cells, strings.ReplaceAll(inlineText(cell), "|", "\\|"))
				}
			}
			*rows = append(*rows, cells)
			continue
		}
		collectRows(c, rows)
	}
}

func htmlBody(doc *html.Node) *html.Node { return findElement(doc, atom.Body) }

func findElement(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, a); found != nil {
			return found
		}
	}
	return nil
}

func firstElement(n *html.Node, a atom.Atom) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == a {
			return c
		}
	}
	return nil
}

// rawText concatenates descendant text without collapsing whitespace.
func rawText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				sb.WriteString(c.Data)
			case html.ElementNode:
				if !htmlDropped[c.DataAtom] {
					walk(c)
				}
			}
		}
	}
	walk(n)
	return sb.String()
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func collapseSpace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(prefix+l, " ")
	}
	return strings.Join(lines, "\n")
}
