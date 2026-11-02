package cli

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"slices"

	"github.com/rasros/lx/pkg/lx"
)

// resolveSections collects every file op once, before rendering, so the tree
// builder and the content pass share one view of what each path refers to.
// It returns how many paths could not be resolved.
//
// Stdin ("-") resolves to nothing: it is a buffer, not a discoverable path, and
// it can only be read once. The content pass handles it directly.
//
// A piped list names a set of paths, so a file it reaches twice, named and
// through its directory, is collected once.
//
// It stops at the first target that finds ctx done and returns ctx's error.
func resolveSections(ctx context.Context, sections []Section, c *lx.Collector) (int, error) {
	unresolved := 0
	piped := map[string]bool{}
	for si := range sections {
		s := &sections[si]
		for oi, op := range s.Ops {
			if op.Action != "FILE" || op.Value == "-" {
				continue
			}
			opts := s.Discover
			opts.Piped = op.Piped
			files, err := c.Collect(ctx, op.Value, opts)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return unresolved, ctxErr
			}
			if err != nil {
				logUnresolved(op.Value, err)
				unresolved++
				continue
			}
			if op.Piped {
				files = slices.DeleteFunc(files, func(f lx.InputFile) bool {
					seen := piped[f.AbsPath]
					piped[f.AbsPath] = true
					return seen
				})
			}
			if len(files) == 0 {
				continue
			}
			if s.resolved == nil {
				s.resolved = make(map[int][]lx.InputFile)
			}
			s.resolved[oi] = files
		}
	}
	return unresolved, nil
}

func logUnresolved(target string, err error) {
	var pe *fs.PathError
	var ue *lx.URLError
	switch {
	case errors.As(err, &pe) && pe.Op == "stat":
		slog.Error("Failed to stat path", "path", pe.Path, "error", err)
	case errors.As(err, &ue) && ue.Download:
		slog.Error("Failed to download URL archive", "url", ue.URL, "error", ue.Err)
	case errors.As(err, &ue):
		slog.Error("Failed to create URL input", "url", ue.URL, "error", ue.Err)
	default:
		slog.Error("Failed to resolve path", "path", target, "error", err)
	}
}
