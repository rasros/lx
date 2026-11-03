package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/rasros/lx/pkg/lx"
)

// resolvePrompts finds every -P prompt, failing on the first it cannot find,
// and reads those in regular files; it leaves stdin, devices and pipes to be
// read in command-line order while rendering. It runs before any input is collected, so
// a bad prompt fails before the walks and downloads of the run.
func resolvePrompts(sections []Section, r *promptResolver, env Env) error {
	for _, s := range sections {
		for _, it := range s.Items {
			p, ok := it.(*promptFileItem)
			if !ok {
				continue
			}
			path, err := r.resolve(p.ref)
			if err != nil {
				return err
			}
			p.path = path
			// A directory fails to read, so it fails here too.
			if info, err := env.stat(path); err == nil && (info.Mode().IsRegular() || info.IsDir()) {
				if err := p.readText(env); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// readText reads the text of p, once.
func (p *promptFileItem) readText(env Env) error {
	if p.read {
		return nil
	}
	data, err := env.readFile(p.path)
	if err != nil {
		return fmt.Errorf("read prompt file %s: %w", p.path, err)
	}
	p.text, p.read = string(data), true
	return nil
}

// resolveSections collects every file item once, before rendering, so trees
// and content share one view of what each path refers to. What an item fails
// on is kept with it, for the render pass to add to the stream ahead of its
// files: the stream reports and counts every failure of the run, in output
// order.
//
// Stdin is not collected: it is a buffer, not a discoverable path, and it can
// only be read once. The render pass reads it.
//
// A piped list names a set of paths, so an input it reaches twice, named and
// through its directory, is collected once: a file once, a failure once, and
// a file it already failed on, or that a failure names, not again. Inputs
// and failures share one key, the absolute path, URL or archive entry path.
//
// It stops at the first target that finds ctx done and returns ctx's error.
func resolveSections(ctx context.Context, sections []Section, c *lx.Collector) error {
	piped := map[string]bool{}
	seen := func(key string) bool {
		s := piped[key]
		piped[key] = true
		return s
	}
	for _, s := range sections {
		for _, it := range s.Items {
			f, ok := it.(*fileItem)
			if !ok {
				continue
			}
			opts := s.Discover
			opts.Piped = f.piped
			files, failures, err := c.Collect(ctx, f.arg, opts)
			if err != nil {
				return err
			}
			if f.piped {
				failures = slices.DeleteFunc(failures, func(f lx.Failure) bool { return seen(f.Path) })
				files = slices.DeleteFunc(files, func(f lx.InputFile) bool { return seen(f.AbsPath) })
			}
			f.files, f.failures = files, failures
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
		var ue *lx.URLError
		if !errors.As(f.Err, &ue) {
			log.Error("Failed to read URL", "url", f.Display, "error", f.Err)
			return
		}
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
