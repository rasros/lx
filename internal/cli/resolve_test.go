package cli

import (
	"context"
	"log/slog"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

// Stdin is left to the render pass, and a missing path keeps its failure.
func TestResolveSections_StdinAndMissingPaths(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	missing := &fileItem{arg: "nope.go"}
	sections := []Section{{Items: []item{&stdinItem{}, missing}}}
	if err := resolveSections(context.Background(), sections, lx.NewCollector(t.TempDir(), log)); err != nil {
		t.Fatal(err)
	}
	if len(missing.files) != 0 {
		t.Errorf("resolved %v, want nothing", missing.files)
	}
	if f := missing.failures; len(f) != 1 || f[0].Kind != lx.FailedStat {
		t.Errorf("failures %v, want one stat failure", f)
	}
}
