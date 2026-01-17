package lx

import (
	"bytes"
	"testing"
)

func TestLayoutWriter(t *testing.T) {
	cfg := NewConfig()
	cfg.SectionHeaderTemplate = "HEAD {{.Index}}\n"
	cfg.SectionFooterTemplate = "FOOT {{.Index}}\n"
	engine, err := CompileTemplates(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sections := []*SectionContext{
		{Index: 0},
		{Index: 1},
	}

	t.Run("Standard Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := newLayoutWriter(&buf, engine, sections, true)

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("Item1"),
			sectionIndex: 0,
			isCompact:    false,
		})

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("Item2"),
			sectionIndex: 0,
			isCompact:    false,
		})

		lw.Close()

		got := buf.String()
		want := "HEAD 0\nItem1\n\nItem2FOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("Compact Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := newLayoutWriter(&buf, engine, sections, true)

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("Item1"),
			sectionIndex: 0,
			isCompact:    true,
		})

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("Item2"),
			sectionIndex: 0,
			isCompact:    true,
		})

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("Item3"),
			sectionIndex: 0,
			isCompact:    false,
		})

		lw.Close()

		got := buf.String()
		want := "HEAD 0\nItem1\nItem2\n\nItem3FOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("Section Transition", func(t *testing.T) {
		var buf bytes.Buffer
		lw := newLayoutWriter(&buf, engine, sections, true)

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("A"),
			sectionIndex: 0,
		})

		lw.WriteItem(result{
			buffer:       bytes.NewBufferString("B"),
			sectionIndex: 1,
		})

		lw.Close()

		got := buf.String()
		want := "HEAD 0\nAFOOT 0\n\n\nHEAD 1\nBFOOT 1\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})

	t.Run("HTML No Spacing", func(t *testing.T) {
		var buf bytes.Buffer
		lw := newLayoutWriter(&buf, engine, sections, false)

		lw.WriteItem(result{buffer: bytes.NewBufferString("<div1/>\n"), sectionIndex: 0})
		lw.WriteItem(result{buffer: bytes.NewBufferString("<div2/>\n"), sectionIndex: 0})
		lw.Close()

		got := buf.String()
		want := "HEAD 0\n<div1/>\n<div2/>\nFOOT 0\n"
		if got != want {
			t.Errorf("Got:\n%q\nWant:\n%q", got, want)
		}
	})
}
