package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
	"github.com/rasros/lx/pkg/lx"
	"gopkg.in/yaml.v3"
)

// LoadConfigChain loads config from Default (~/.config/lx/config.yaml) -> Env (LX_CONFIG) -> CLI flags (explicit file).
func LoadConfigChain(cliPath string) (*lx.Config, error) {
	cfg := &lx.Config{}

	// 1. User config (default location)
	if configDir, err := os.UserConfigDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = mergeConfigFile(cfg, path, false)
	}

	// 2. Env variable
	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		if err := mergeConfigFile(cfg, envPath, false); err != nil {
			return nil, fmt.Errorf("load env config: %w", err)
		}
	}

	// 3. CLI explicit path
	if cliPath != "" {
		if err := mergeConfigFile(cfg, cliPath, true); err != nil {
			return nil, fmt.Errorf("load cli config: %w", err)
		}
	}

	// 4. Load Global Ignores (side-effect on cfg.GlobalIgnore)
	cfg.GlobalIgnore = loadGlobalIgnores()

	return cfg, nil
}

func mergeConfigFile(cfg *lx.Config, path string, strict bool) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		if strict {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	loaded := &lx.Config{}
	if err := yaml.NewDecoder(f).Decode(loaded); err != nil {
		return err
	}

	lx.Merge(cfg, loaded)
	cfg.LoadedConfigs = append(cfg.LoadedConfigs, path)
	return nil
}

func loadGlobalIgnores() gitignore.IgnoreMatcher {
	var lines []string

	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()

	candidates := []string{
		filepath.Join(configDir, "lx", "ignore"),
	}

	// XDG Support for global gitignore
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	if xdgConfig != "" {
		candidates = append(candidates, filepath.Join(xdgConfig, "git", "ignore"))
	}

	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			lines = append(lines, strings.Split(string(data), "\n")...)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	buf := bytes.NewBufferString(strings.Join(lines, "\n"))
	return gitignore.NewGitIgnoreFromReader(".", buf)
}
