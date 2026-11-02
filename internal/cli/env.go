package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/atotto/clipboard"
	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/internal/userdir"
)

// Env is everything a run sees of the process it runs in, so that runs can be
// driven side by side, as the golden tests do. Every field must be set;
// Clipboard is only called when the output is copied.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// LookupEnv looks an environment variable up, as os.LookupEnv does.
	LookupEnv func(key string) (string, bool)
	// Cwd is the absolute working directory relative paths resolve against.
	Cwd string
	// Clipboard receives the output of a run that copies it.
	Clipboard func(text string) error
}

// OSEnv is the Env of the running process.
func OSEnv() (Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Env{}, fmt.Errorf("read working directory: %w", err)
	}
	return Env{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		LookupEnv: os.LookupEnv,
		Cwd:       cwd,
		Clipboard: clipboard.WriteAll,
	}, nil
}

// ExitError is an error Run has already reported; the process exits with
// Code without reporting it again.
type ExitError struct{ Code int }

func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// getenv returns the value of the variable key, or "" when it is unset.
func (e Env) getenv(key string) string {
	v, _ := e.LookupEnv(key)
	return v
}

// abs returns p as an absolute path, resolved against the working directory.
func (e Env) abs(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(e.Cwd, p)
}

// The file operations below resolve name against the working directory and,
// as the os functions given name would, report errors with name as given.

func (e Env) stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(e.abs(name))
	return info, asGiven(err, name)
}

func (e Env) open(name string) (*os.File, error) {
	f, err := os.Open(e.abs(name))
	return f, asGiven(err, name)
}

func (e Env) create(name string) (*os.File, error) {
	f, err := os.Create(e.abs(name))
	return f, asGiven(err, name)
}

func (e Env) readFile(name string) ([]byte, error) {
	data, err := os.ReadFile(e.abs(name))
	return data, asGiven(err, name)
}

// asGiven names name in err, a *fs.PathError about it.
func asGiven(err error, name string) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		pe.Path = name
	}
	return err
}

// paint returns the Painter for output written to w.
func (e Env) paint(w io.Writer) ansi.Painter { return ansi.For(w, e.LookupEnv) }

func (e Env) homeDir() (string, error)   { return userdir.Home(e.LookupEnv) }
func (e Env) configDir() (string, error) { return userdir.Config(e.LookupEnv) }
