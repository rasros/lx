package cli

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/rasros/lx/pkg/lx"
)

// resolveSections collects every file op once, before rendering, so the tree
// builder and the content pass share one view of what each path refers to.
// What an op fails on is kept with it, for the content pass to add to the
// stream ahead of the op's files: the stream reports and counts every failure
// of the run, in output order.
//
// Stdin ("-") resolves to nothing: it is a buffer, not a discoverable path, and
// it can only be read once. The content pass handles it directly.
//
// A piped list names a set of paths, so a file it reaches twice, named and
// through its directory, is collected once.
//
// It stops at the first target that finds ctx done and returns ctx's error.
func resolveSections(ctx context.Context, sections []Section, c *lx.Collector) error {
	piped := map[string]bool{}
	pipedFailures := map[string]bool{}
	for si := range sections {
		s := &sections[si]
		for oi, op := range s.Ops {
			if op.Action != "FILE" || op.Value == "-" {
				continue
			}
			opts := s.Discover
			opts.Piped = op.Piped
			files, failures, err := c.Collect(ctx, op.Value, opts)
			if err != nil {
				return err
			}
			if op.Piped {
				failures = slices.DeleteFunc(failures, func(f lx.Failure) bool {
					seen := pipedFailures[f.Path]
					pipedFailures[f.Path] = true
					return seen
				})
			}
			if len(failures) > 0 {
				if s.failures == nil {
					s.failures = make(map[int][]lx.Failure)
				}
				s.failures[oi] = failures
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
	return nil
}

// stdinPath is the Failure.Path of stdin.
const stdinPath = "-"

// logFailure logs an input the run could not read, naming the step it
// failed at.
func logFailure(log *slog.Logger, f lx.Failure) {
	switch f.Kind {
	case lx.FailedStat:
		log.Error("Failed to stat path", "path", f.Display, "error", f.Err)
	case lx.FailedURL:
		// Collect's URL failures are *URLErrors.
		var ue *lx.URLError
		errors.As(f.Err, &ue)
		msg := "Failed to create URL input"
		if ue.Download {
			msg = "Failed to download URL archive"
		}
		log.Error(msg, "url", ue.URL, "error", ue.Err)
	case lx.FailedWalk:
		log.Error("Failed to access path during walk", "path", f.Display, "error", f.Err)
	case lx.FailedArchive:
		log.Error("Failed to expand archive", "path", f.Display, "error", f.Err)
	case lx.FailedArchiveEntry:
		log.Error("Failed to read archive entry", "path", f.Display, "error", f.Err)
	default:
		if f.Path == stdinPath {
			log.Error("Failed to read from stdin", "error", f.Err)
			return
		}
		log.Error("Failed to read file", "path", f.Display, "error", f.Err)
	}
}
