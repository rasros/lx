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
	Index int
	// Buffer holds what the item rendered; nil for an added failure and for
	// a section title, which the LayoutWriter renders.
	Buffer *bytes.Buffer
	// Section marks the item that starts an explicit section.
	Section      bool
	Stats        int64
	Tokens       int64
	Rows         int
	IsCompact    bool
	IsBinary     bool
	Err          error
	SectionIndex int
	File         sources.InputFile
	// Failure is the input the item failed on, if any.
	Failure *core.Failure
	// Convert is a converter that failed on File, leaving its raw content.
	Convert *ConvertFailure
}

// LayoutWriter writes pipeline results to the output, adding each section's
// header, title and footer.
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

// WriteItem handles a single rendered item from the pipeline. A Section
// item writes only the start of its section.
func (lw *LayoutWriter) WriteItem(res Result) error {
	if res.SectionIndex != lw.currentSecIdx {
		if err := lw.handleSectionChange(res.SectionIndex); err != nil {
			return err
		}
	}
	if res.Section {
		return nil
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

// handleSectionChange closes the current section and opens section newIdx:
// its header, then the title of an explicit section, which counts as written
// for the separator before the next item.
func (lw *LayoutWriter) handleSectionChange(newIdx int) error {
	if err := lw.closeSection(); err != nil {
		return err
	}

	if lw.hasWritten && lw.enableSeparators {
		if err := lw.writeCounted([]byte("\n\n")); err != nil {
			return err
		}
	}

	lw.currentSecIdx = newIdx
	lw.hasWritten = false
	lw.lastIsCompact = false
	sec := lw.section(newIdx)
	if sec == nil {
		return nil
	}
	if err := lw.renderTemplate(lw.engine.SectionHeader, sec); err != nil {
		return err
	}
	if !sec.IsImplicit {
		if err := lw.renderTemplate(lw.engine.Section, sec); err != nil {
			return err
		}
		lw.hasWritten = true
	}
	return nil
}

// Close ensures the final section is properly closed with a footer.
func (lw *LayoutWriter) Close() error {
	return lw.closeSection()
}

// closeSection writes the footer of the current section, if there is one.
func (lw *LayoutWriter) closeSection() error {
	if sec := lw.section(lw.currentSecIdx); sec != nil {
		return lw.renderTemplate(lw.engine.SectionFooter, sec)
	}
	return nil
}

// section returns the section of index idx, or nil.
func (lw *LayoutWriter) section(idx int) *core.SectionContext {
	for _, s := range lw.sections {
		if s.Index == idx {
			return s
		}
	}
	return nil
}

func (lw *LayoutWriter) renderTemplate(tmpl *template.Template, sec *core.SectionContext) error {
	tokens, err := ExecuteCounted(lw.w, tmpl, sec, lw.tokenizer)
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
