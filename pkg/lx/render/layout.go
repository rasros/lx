package render

import (
	"bytes"
	"io"
	"text/template"

	"github.com/rasros/lx/pkg/lx/core"
)

// Result is one rendered item produced by the worker pipeline.
type Result struct {
	Index        int
	Buffer       *bytes.Buffer
	Stats        int64
	Rows         int
	IsCompact    bool
	IsBinary     bool
	IsError      bool
	Err          error
	SectionIndex int
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
		if _, err := lw.w.Write([]byte(sep)); err != nil {
			return err
		}
	}

	if _, err := lw.w.Write(res.Buffer.Bytes()); err != nil {
		return err
	}

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
		if _, err := lw.w.Write([]byte("\n\n")); err != nil {
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
	return tmpl.Execute(lw.w, ctx)
}
