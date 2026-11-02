package render

import (
	"bytes"
	"io"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

// Result is one rendered item produced by the worker pipeline.
type Result struct {
	Index        int
	Buffer       *bytes.Buffer
	Stats        int64
	Tokens       int64
	Rows         int
	IsCompact    bool
	IsBinary     bool
	IsError      bool
	Err          error
	SectionIndex int
	Issues       FileIssues // of File, a file item
	File         sources.InputFile
}

// LayoutWriter acts as a middleman between the pipeline assembler and the output writer.
type LayoutWriter struct {
	w                io.Writer
	engine           *core.TemplateEngine
	sections         []*core.SectionContext
	enableSeparators bool
	currentSecIdx    int
	hasWritten       bool
	lastIsCompact    bool
	tokenizer        core.Tokenizer
	tokens           int64
}

func NewLayoutWriter(w io.Writer, engine *core.TemplateEngine, sections []*core.SectionContext, enableSeparators bool) *LayoutWriter {
	return &LayoutWriter{
		w:                w,
		engine:           engine,
		sections:         sections,
		enableSeparators: enableSeparators,
		currentSecIdx:    -1,
	}
}

// SetTokenizer makes the writer estimate tokens for what it writes besides the
// items, whose estimates arrive with them in Result.Tokens.
func (lw *LayoutWriter) SetTokenizer(t core.Tokenizer) {
	lw.tokenizer = t
}

// Tokens is the estimate for everything written so far.
func (lw *LayoutWriter) Tokens() int64 {
	return lw.tokens
}

// WriteItem handles a single rendered item from the pipeline.
func (lw *LayoutWriter) WriteItem(res Result) error {
	if res.SectionIndex != lw.currentSecIdx {
		if err := lw.handleSectionChange(res.SectionIndex); err != nil {
			return err
		}
	}

	if lw.hasWritten && lw.enableSeparators {
		sep := "\n\n"
		if lw.lastIsCompact && res.IsCompact {
			sep = "\n"
		}
		if err := lw.writeCounted([]byte(sep)); err != nil {
			return err
		}
	}

	if _, err := lw.w.Write(res.Buffer.Bytes()); err != nil {
		return err
	}
	lw.tokens += res.Tokens

	lw.hasWritten = true
	lw.lastIsCompact = res.IsCompact
	return nil
}

func (lw *LayoutWriter) handleSectionChange(newIdx int) error {
	if lw.currentSecIdx >= 0 {
		if err := lw.renderTemplate(lw.engine.SectionFooter, lw.currentSecIdx); err != nil {
			return err
		}
	}

	if lw.hasWritten && lw.enableSeparators {
		if err := lw.writeCounted([]byte("\n\n")); err != nil {
			return err
		}
	}

	if err := lw.renderTemplate(lw.engine.SectionHeader, newIdx); err != nil {
		return err
	}

	lw.currentSecIdx = newIdx
	lw.hasWritten = false
	lw.lastIsCompact = false
	return nil
}

// Close ensures the final section is properly closed with a footer.
func (lw *LayoutWriter) Close() error {
	if lw.currentSecIdx >= 0 {
		return lw.renderTemplate(lw.engine.SectionFooter, lw.currentSecIdx)
	}
	return nil
}

func (lw *LayoutWriter) renderTemplate(tmpl *template.Template, secIdx int) error {
	var ctx core.SectionContext
	found := false
	for _, s := range lw.sections {
		if s.Index == secIdx {
			ctx = *s
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	tokens, err := ExecuteCounted(lw.w, tmpl, ctx, lw.tokenizer)
	lw.tokens += tokens
	return err
}

// writeCounted writes p and adds its token estimate.
func (lw *LayoutWriter) writeCounted(p []byte) error {
	if lw.tokenizer != nil {
		lw.tokens += lw.tokenizer.Estimate(int64(len(p)), p)
	}
	_, err := lw.w.Write(p)
	return err
}

// ExecuteCounted renders tmpl to w in one write and estimates its tokens over
// the whole output, so the estimate does not depend on how the template splits
// its writes. A nil tokenizer estimates nothing.
func ExecuteCounted(w io.Writer, tmpl *template.Template, data interface{}, tokenizer core.Tokenizer) (int64, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return 0, err
	}
	var tokens int64
	if tokenizer != nil && buf.Len() > 0 {
		tokens = tokenizer.Estimate(int64(buf.Len()), buf.Bytes())
	}
	_, err := w.Write(buf.Bytes())
	return tokens, err
}
