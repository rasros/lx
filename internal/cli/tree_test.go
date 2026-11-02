package cli

import (
	"context"
	"testing"

	"github.com/rasros/lx/pkg/lx"
)

// Stdin is left to the content pass, and a missing path is counted so the
// stats summary can report it.
func TestResolveSections_StdinAndMissingPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	sections := []Section{{Ops: []Op{
		{Action: "FILE", Value: "-", Type: CmdAction},
		{Action: "FILE", Value: "nope.go", Type: CmdAction},
	}}}
	if got, err := resolveSections(context.Background(), sections, lx.NewCollector("", nil)); got != 1 || err != nil {
		t.Errorf("unresolved = %d, %v; want 1, nil", got, err)
	}
	if len(sections[0].resolved) != 0 {
		t.Errorf("resolved %v, want nothing", sections[0].resolved)
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

	assignTrees(groups)
	if !groups[0].skipFileOps[1] {
		t.Fatalf("expected file op at index 1 to be skipped in tree-only section")
	}
}
