package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/rasros/lx/internal/ansi"
)

type CliHandler struct {
	w     io.Writer
	mu    sync.Mutex
	level slog.Level
	attrs []slog.Attr
	paint ansi.Painter
}

// NewCliHandler writes log records to w, coloring the level prefix with
// paint.
func NewCliHandler(w io.Writer, level slog.Level, paint ansi.Painter) *CliHandler {
	return &CliHandler{
		w:     w,
		level: level,
		paint: paint,
	}
}

func (h *CliHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *CliHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var sb strings.Builder

	switch r.Level {
	case slog.LevelError:
		sb.WriteString(h.paint.Paint("Error:", ansi.Bold, ansi.Red) + " ")
	case slog.LevelWarn:
		sb.WriteString(h.paint.Paint("Warning:", ansi.Bold, ansi.Yellow) + " ")
	case slog.LevelInfo:
		sb.WriteString("Info: ")
	}

	sb.WriteString(r.Message)

	var errorStr string
	var otherAttrs []string

	processAttr := func(a slog.Attr) {
		if a.Key == "error" || a.Key == "err" {
			errorStr = a.Value.String()
			return
		}
		if a.Key == "" {
			return
		}
		otherAttrs = append(otherAttrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
	}

	for _, a := range h.attrs {
		processAttr(a)
	}

	r.Attrs(func(a slog.Attr) bool {
		processAttr(a)
		return true
	})

	if errorStr != "" {
		sb.WriteString(": ")
		sb.WriteString(errorStr)
	}

	if len(otherAttrs) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(otherAttrs, " "))
		sb.WriteString(")")
	}

	sb.WriteString("\n")

	_, err := fmt.Fprint(h.w, sb.String())
	return err
}

func (h *CliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &CliHandler{
		w:     h.w,
		level: h.level,
		attrs: newAttrs,
		paint: h.paint,
	}
}

func (h *CliHandler) WithGroup(name string) slog.Handler {
	return h
}
