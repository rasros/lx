package osenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Through os.LookupEnv the directories are the ones os finds.
func TestMatchesOS(t *testing.T) {
	home, homeErr := Home(os.LookupEnv)
	osHome, osHomeErr := os.UserHomeDir()
	if home != osHome || (homeErr == nil) != (osHomeErr == nil) {
		t.Errorf("Home = %q, %v; os.UserHomeDir = %q, %v", home, homeErr, osHome, osHomeErr)
	}
	config, configErr := Config(os.LookupEnv)
	osConfig, osConfigErr := os.UserConfigDir()
	if config != osConfig || (configErr == nil) != (osConfigErr == nil) {
		t.Errorf("Config = %q, %v; os.UserConfigDir = %q, %v", config, configErr, osConfig, osConfigErr)
	}
}

func TestReadsOnlyTheGivenEnv(t *testing.T) {
	env := func(k string) (string, bool) {
		v, ok := map[string]string{"HOME": "/h", "USERPROFILE": "/h", "home": "/h", "AppData": "/h/AppData"}[k]
		return v, ok
	}
	if home, err := Home(env); err != nil || home != "/h" {
		t.Errorf("Home = %q, %v; want /h", home, err)
	}
	if config, err := Config(env); err != nil || config == "" {
		t.Errorf("Config = %q, %v; want a directory below /h", config, err)
	}
}

// A relative $XDG_CONFIG_HOME is ignored, as the XDG spec says.
func TestRelativeXDGConfigHomeIsIgnored(t *testing.T) {
	env := func(k string) (string, bool) {
		v, ok := map[string]string{"HOME": "/h", "USERPROFILE": "/h", "home": "/h", "XDG_CONFIG_HOME": "rel"}[k]
		return v, ok
	}
	if got, want := XDGConfigHome(env), filepath.Join("/h", ".config"); got != want {
		t.Errorf("XDGConfigHome = %q, want %q", got, want)
	}
	if runtime.GOOS == "linux" {
		if got, err := Config(env); err != nil || got != filepath.Join("/h", ".config") {
			t.Errorf("Config = %q, %v; want ~/.config", got, err)
		}
	}
}
