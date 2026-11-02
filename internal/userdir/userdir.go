// Package userdir finds the user's home and config directories from an
// environment given as a lookup function, the way os.UserHomeDir and
// os.UserConfigDir find them from the process environment.
package userdir

import (
	"errors"
	"path/filepath"
	"runtime"
)

// LookupEnv looks an environment variable up, as os.LookupEnv does.
type LookupEnv func(key string) (string, bool)

func get(env LookupEnv, key string) string {
	v, _ := env(key)
	return v
}

// Home returns the user's home directory, as os.UserHomeDir does.
func Home(env LookupEnv) (string, error) {
	key := "HOME"
	switch runtime.GOOS {
	case "windows":
		key = "USERPROFILE"
	case "plan9":
		key = "home"
	}
	if v := get(env, key); v != "" {
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
func Config(env LookupEnv) (string, error) {
	switch runtime.GOOS {
	case "windows":
		if dir := get(env, "AppData"); dir != "" {
			return dir, nil
		}
		return "", errors.New("%AppData% is not defined")
	case "darwin", "ios":
		home := get(env, "HOME")
		if home == "" {
			return "", errors.New("$HOME is not defined")
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "plan9":
		home := get(env, "home")
		if home == "" {
			return "", errors.New("$home is not defined")
		}
		return filepath.Join(home, "lib"), nil
	}
	if dir := get(env, "XDG_CONFIG_HOME"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", errors.New("path in $XDG_CONFIG_HOME is relative")
		}
		return dir, nil
	}
	home := get(env, "HOME")
	if home == "" {
		return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined")
	}
	return filepath.Join(home, ".config"), nil
}
