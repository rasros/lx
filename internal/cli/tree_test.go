package cli

import (
	"context"
	"log/slog"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

// Stdin is left to the content pass, and a missing path is kept with its op
// for the content pass to add to the stream.
func TestResolveSections_StdinAndMissingPaths(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	sections := []Section{{Ops: []Op{
		{Action: "FILE", Value: "-", Type: CmdAction},
		{Action: "FILE", Value: "nope.go", Type: CmdAction},
	}}}
	if err := resolveSections(context.Background(), sections, lx.NewCollector(t.TempDir(), log)); err != nil {
		t.Fatal(err)
	}
	if len(sections[0].resolved) != 0 {
		t.Errorf("resolved %v, want nothing", sections[0].resolved)
	}
	if f := sections[0].failures; len(f) != 1 || len(f[1]) != 1 || f[1][0].Kind != lx.FailedStat {
		t.Errorf("failures %v, want the stat failure of op 1", f)
	}
}

func TestAssignTrees_TreeOnlyMarksFileOps(t *testing.T) {
	groups := []Section{
		{
			Ops: []Op{
				{Action: "tree-only", Type: CmdAction},
				{Action: "FILE", Value: "https://example.com/repo.zip", Type: CmdAction},
			},
		},
	}

	assignTrees(slog.Default(), groups)
	if !groups[0].skipFileOps[1] {
		t.Fatalf("expected file op at index 1 to be skipped in tree-only section")
	}
}
