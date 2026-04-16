package render

import (
	"bytes"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/templatex"
)

func TestLayoutWriter(t *testing.T) {
	cfg := core.NewConfig()
	cfg.SectionHeaderTemplate = "HEAD {{.Index}}\n"
	cfg.SectionFooterTemplate = "FOOT {{.Index}}\n"
	engine, err := templatex.Compile(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sections := []*core.SectionContext{{Index: 0}, {Index: 1}}

	t.Run("Standard Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := NewLayoutWriter(&buf, engine, sections, true)
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("Item1"), SectionIndex: 0, IsCompact: false})
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("Item2"), SectionIndex: 0, IsCompact: false})
		_ = lw.Close()

		got := buf.String()
		want := "HEAD 0\nItem1\n\nItem2FOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("Compact Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := NewLayoutWriter(&buf, engine, sections, true)
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("Item1"), SectionIndex: 0, IsCompact: true})
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("Item2"), SectionIndex: 0, IsCompact: true})
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("Item3"), SectionIndex: 0, IsCompact: false})
		_ = lw.Close()

		got := buf.String()
		want := "HEAD 0\nItem1\nItem2\n\nItem3FOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("Section Transition", func(t *testing.T) {
		var buf bytes.Buffer
		lw := NewLayoutWriter(&buf, engine, sections, true)
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("A"), SectionIndex: 0})
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("B"), SectionIndex: 1})
		_ = lw.Close()

		got := buf.String()
		want := "HEAD 0\nAFOOT 0\n\n\nHEAD 1\nBFOOT 1\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("HTML No Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := NewLayoutWriter(&buf, engine, sections, false)
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("<div1/>\n"), SectionIndex: 0})
		_ = lw.WriteItem(Result{Buffer: bytes.NewBufferString("<div2/>\n"), SectionIndex: 0})
		_ = lw.Close()

		got := buf.String()
		want := "HEAD 0\n<div1/>\n<div2/>\nFOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})
}
