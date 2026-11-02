package cli

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// fileSink is where -o writes. A regular file, or one still to be created,
// only receives the output once the run has succeeded, so a failed run
// leaves it as it was: the output goes to a buffer, an anonymous temporary
// file, until Commit writes it to the file as os.Create opens it. Anything
// else, such as a device, a pipe or /dev/stdout, is opened up front and
// written to as the output is produced.
type fileSink struct {
	f        *os.File // the buffer, or the target itself
	buffered bool
	unlinked bool // the buffer has no name left to remove
	path     string
	name     string // the path as given, for errors
	done     bool
}

// createFileSink checks that the file at path, named name in errors, can be
// written, as os.Create would find, without creating it, and starts the
// sink. Buffers are made in tempDir.
func createFileSink(path, name, tempDir string) (*fileSink, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return nil, &fs.PathError{Op: "open", Path: name, Err: syscall.EISDIR}
	case err == nil && !info.Mode().IsRegular():
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			return nil, asGiven(err, name)
		}
		return &fileSink{f: f, path: path, name: name}, nil
	case err == nil:
		if err := checkWritable(path); err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	case errors.Is(err, fs.ErrNotExist):
		if err := checkCreatable(path); err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	default:
		return nil, asGiven(err, name)
	}

	buf, err := os.CreateTemp(tempDir, "lx-output-*")
	if err != nil {
		return nil, err
	}
	s := &fileSink{f: buf, buffered: true, path: path, name: name}
	// Where an open file can lose its name, it does at once, so nothing is
	// left behind however the process ends.
	s.unlinked = os.Remove(buf.Name()) == nil
	return s, nil
}

// checkCreatable reports why os.Create could not create the missing file at
// path, through a dangling symlink too: its directory must exist and be
// writable.
func checkCreatable(path string) error {
	target := path
	if link, err := os.Readlink(path); err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		target = link
	}
	dir := filepath.Dir(target)
	info, err := os.Stat(dir)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return pe.Err
		}
		return err
	}
	if !info.IsDir() {
		return syscall.ENOTDIR
	}
	return checkWritable(dir)
}

func (s *fileSink) Write(p []byte) (int, error) { return s.f.Write(p) }

// Commit finishes the output: a buffered one is written to the file.
func (s *fileSink) Commit() error {
	if s.done {
		return nil
	}
	if !s.buffered {
		s.done = true
		return s.f.Close()
	}
	defer s.Abort()
	if _, err := s.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	out, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return asGiven(err, s.name)
	}
	_, err = io.Copy(out, s.f)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Abort drops a buffered output, leaving the file untouched. It does nothing
// after Commit.
func (s *fileSink) Abort() {
	if s.done {
		return
	}
	s.done = true
	s.f.Close()
	if s.buffered && !s.unlinked {
		os.Remove(s.f.Name())
	}
}
