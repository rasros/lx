package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// fileSink writes the output file of -o to a temporary file beside it, which
// replaces the file only on Commit. Until then, and after Abort, the file is
// as the run found it, so a failed run never leaves it truncated or partial.
type fileSink struct {
	f      *os.File
	target string
	done   bool
}

// createFileSink starts writing the file at path, an absolute path, named
// name in errors. A symlink at path is followed, so the file it points to is
// the one replaced.
func createFileSink(path, name string) (*fileSink, error) {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	perm := fs.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return nil, &fs.PathError{Op: "open", Path: name, Err: syscall.EISDIR}
		}
		perm = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.tmp")
	if err != nil {
		// Name the output file, not the temporary one, as creating it would.
		if pe := (*fs.PathError)(nil); errors.As(err, &pe) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: pe.Err}
		}
		return nil, err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return &fileSink{f: f, target: target}, nil
}

func (s *fileSink) Write(p []byte) (int, error) { return s.f.Write(p) }

// Commit replaces the output file with what was written.
func (s *fileSink) Commit() error {
	if s.done {
		return nil
	}
	s.done = true
	if err := s.f.Close(); err != nil {
		os.Remove(s.f.Name())
		return err
	}
	if err := os.Rename(s.f.Name(), s.target); err != nil {
		os.Remove(s.f.Name())
		return err
	}
	return nil
}

// Abort drops what was written, leaving the output file untouched. It does
// nothing after Commit.
func (s *fileSink) Abort() {
	if s.done {
		return
	}
	s.done = true
	s.f.Close()
	os.Remove(s.f.Name())
}
