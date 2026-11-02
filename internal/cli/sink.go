package cli

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// fileSink is the file -o names. A run writes its output to a buffer, a
// temporary file elsewhere, and the file only receives it on Commit, so a run
// that fails leaves the file as it found it.
//
// Commit replaces a regular file the run owns, with one link, by renaming a
// copy beside it, keeping its mode. Anything else is written in place, as
// os.Create would: a missing file is created with the umask, and symlinks,
// devices, pipes, files with other links or owners and files in directories
// the run cannot write to are written through.
type fileSink struct {
	path string // absolute
	name string // as given, for errors
	buf  *os.File
	done bool
}

// createFileSink checks that the file at path, named name in errors, could be
// created, without creating it, and starts buffering its content.
func createFileSink(path, name string) (*fileSink, error) {
	if err := checkOutputPath(path, name); err != nil {
		return nil, err
	}
	buf, err := os.CreateTemp("", "lx-output-*")
	if err != nil {
		return nil, err
	}
	return &fileSink{path: path, name: name, buf: buf}, nil
}

// checkOutputPath reports the error os.Create would return for path, short of
// those only trying to write it finds, such as a read-only directory.
func checkOutputPath(path, name string) error {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return &fs.PathError{Op: "open", Path: name, Err: syscall.EISDIR}
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return asGiven(err, name)
	}
	// os.Create creates a missing file, through a dangling symlink too, in a
	// directory that must exist.
	target := path
	if link, err := os.Readlink(path); err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		target = link
	}
	dir, err := os.Stat(filepath.Dir(target))
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return &fs.PathError{Op: "open", Path: name, Err: pe.Err}
		}
		return err
	}
	if !dir.IsDir() {
		return &fs.PathError{Op: "open", Path: name, Err: syscall.ENOTDIR}
	}
	return nil
}

func (s *fileSink) Write(p []byte) (int, error) { return s.buf.Write(p) }

// Commit writes what was buffered to the file.
func (s *fileSink) Commit() error {
	if s.done {
		return nil
	}
	defer s.Abort()
	if _, err := s.buf.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if info, err := os.Lstat(s.path); err == nil && info.Mode().IsRegular() && soleOwnedLink(info) {
		replaced, err := s.replace(info)
		if replaced || err != nil {
			return err
		}
		// The directory cannot take a new file; write in place.
		if _, err := s.buf.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}
	return s.writeInPlace()
}

// replace renames a copy of the buffer over the file, which info describes,
// reporting false when the directory does not let it make one.
func (s *fileSink) replace(info fs.FileInfo) (bool, error) {
	// A short fixed name, whatever the length of the file's own.
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".lx-*.tmp")
	if errors.Is(err, fs.ErrPermission) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	_, err = io.Copy(tmp, s.buf)
	if err == nil {
		err = tmp.Chmod(info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky))
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), s.path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return true, err
	}
	return true, nil
}

// writeInPlace writes the buffer to the file as os.Create opens it. A file it
// creates is removed again if the write fails.
func (s *fileSink) writeInPlace() error {
	_, statErr := os.Stat(s.path)
	created := errors.Is(statErr, fs.ErrNotExist)
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return asGiven(err, s.name)
	}
	_, err = io.Copy(f, s.buf)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil && created {
		os.Remove(s.path)
	}
	return err
}

// Abort drops what was buffered, leaving the file untouched. It does nothing
// after Commit.
func (s *fileSink) Abort() {
	if s.done {
		return
	}
	s.done = true
	s.buf.Close()
	os.Remove(s.buf.Name())
}
