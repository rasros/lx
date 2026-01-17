package lx

import (
	"io"
	"text/template"
)

// layoutWriter acts as a middleman between the pipeline assembler and the output writer.
// It manages section transitions and dynamic spacing between items.
//
// It is an internal helper for Stream.Execute.
type layoutWriter struct {
	w                io.Writer
	engine           *TemplateEngine
	sections         []*SectionContext
	enableSeparators bool
	currentSecIdx    int
	hasWritten       bool
	lastIsCompact    bool
}

func newLayoutWriter(w io.Writer, engine *TemplateEngine, sections []*SectionContext, enableSeparators bool) *layoutWriter {
	return &layoutWriter{
		w:                w,
		engine:           engine,
		sections:         sections,
		enableSeparators: enableSeparators,
		currentSecIdx:    -1,
	}
}

// WriteItem handles the logic for a single rendered item from the pipeline.
func (lw *layoutWriter) WriteItem(res result) error {
	if res.sectionIndex != lw.currentSecIdx {
		if err := lw.handleSectionChange(res.sectionIndex); err != nil {
			return err
		}
	}

	if lw.hasWritten && lw.enableSeparators {
		sep := "\n\n"
		if lw.lastIsCompact && res.isCompact {
			sep = "\n"
		}
		if _, err := lw.w.Write([]byte(sep)); err != nil {
			return err
		}
	}

	if _, err := lw.w.Write(res.buffer.Bytes()); err != nil {
		return err
	}

	lw.hasWritten = true
	lw.lastIsCompact = res.isCompact
	return nil
}

func (lw *layoutWriter) handleSectionChange(newIdx int) error {
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
func (lw *layoutWriter) Close() error {
	if lw.currentSecIdx >= 0 {
		return lw.renderTemplate(lw.engine.SectionFooter, lw.currentSecIdx)
	}
	return nil
}

func (lw *layoutWriter) renderTemplate(tmpl *template.Template, secIdx int) error {
	var ctx SectionContext
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
