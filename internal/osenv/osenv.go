// Package osenv is the environment of a run as a lookup function, and the
// user's home and config directories found through it.
package osenv

import (
	"errors"
	"path/filepath"
	"runtime"
)

// Lookup looks an environment variable up, as os.LookupEnv does.
type Lookup func(key string) (string, bool)

// Get returns the value of key, or "" when it is unset.
func (env Lookup) Get(key string) string {
	v, _ := env(key)
	return v
}

// Home returns the user's home directory, as os.UserHomeDir does.
func Home(env Lookup) (string, error) {
	key := "HOME"
	switch runtime.GOOS {
	case "windows":
		key = "USERPROFILE"
	case "plan9":
		key = "home"
	}
	if v := env.Get(key); v != "" {
		return v, nil
	}
	switch runtime.GOOS {
	case "android":
		return "/sdcard", nil
	case "ios":
		return "/", nil
	}
	return "", errors.New("$" + key + " is not defined")
}

// Config returns the user's config directory, as os.UserConfigDir does, but
// for a relative $XDG_CONFIG_HOME, which it ignores as the XDG spec says
// instead of failing.
func Config(env Lookup) (string, error) {
	switch runtime.GOOS {
	case "windows":
		if dir := env.Get("AppData"); dir != "" {
			return dir, nil
		}
		return "", errors.New("%AppData% is not defined")
	case "darwin", "ios":
		home := env.Get("HOME")
		if home == "" {
			return "", errors.New("$HOME is not defined")
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "plan9":
		home := env.Get("home")
		if home == "" {
			return "", errors.New("$home is not defined")
		}
		return filepath.Join(home, "lib"), nil
	}
	if dir := XDGConfigHome(env); dir != "" {
		return dir, nil
	}
	return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined")
}

// XDGConfigHome returns $XDG_CONFIG_HOME, or ~/.config when it is unset or
// relative, which the XDG spec says to ignore; "" without a home either.
func XDGConfigHome(env Lookup) string {
	if dir := env.Get("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	if home, err := Home(env); err == nil {
		return filepath.Join(home, ".config")
	}
	return ""
}
