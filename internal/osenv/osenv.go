// Package osenv is the environment of a run as a lookup function, and the
// user's home and config directories found through it, the way
// os.UserHomeDir and os.UserConfigDir find them in the process environment.
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

// Config returns the user's config directory, as os.UserConfigDir does.
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
	if dir := env.Get("XDG_CONFIG_HOME"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", errors.New("path in $XDG_CONFIG_HOME is relative")
		}
		return dir, nil
	}
	home := env.Get("HOME")
	if home == "" {
		return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined")
	}
	return filepath.Join(home, ".config"), nil
}
