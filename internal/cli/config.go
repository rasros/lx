package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
	"github.com/rasros/lx/pkg/lx"
	"gopkg.in/yaml.v3"
)

// CliConfig adds CLI-specific metadata on top of the library config.
type CliConfig struct {
	lx.Config  `yaml:",inline"`
	OutputMode string `yaml:"output_mode"`
	ShowStats  string `yaml:"show_stats"`
	Verbosity  string `yaml:"verbosity"`
}

func LoadConfigChain(cliPath string) (*lx.Config, error) {
	cliCfg := &CliConfig{
		Config: *lx.NewConfig(),
	}

	// 1. User config
	if configDir, err := os.UserConfigDir(); err == nil {
		path := filepath.Join(configDir, "lx", "config.yaml")
		_ = mergeConfigFile(cliCfg, path, false)
	}

	// 2. Env
	if envPath := os.Getenv("LX_CONFIG"); envPath != "" {
		_ = mergeConfigFile(cliCfg, envPath, false)
	}

	// 3. CLI explicit
	if cliPath != "" {
		if err := mergeConfigFile(cliCfg, cliPath, true); err != nil {
			return nil, err
		}
	}

	cliCfg.Config.GlobalIgnore = loadGlobalIgnores()
	return &cliCfg.Config, nil
}

func mergeConfigFile(dst *CliConfig, path string, strict bool) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) && !strict {
			return nil
		}
		return err
	}
	defer f.Close()

	var loaded CliConfig
	if err := yaml.NewDecoder(f).Decode(&loaded); err != nil {
		return err
	}

	lx.Merge(&dst.Config, &loaded.Config)
	if loaded.OutputMode != "" {
		dst.OutputMode = loaded.OutputMode
	}
	if loaded.ShowStats != "" {
		dst.ShowStats = loaded.ShowStats
	}
	if loaded.Verbosity != "" {
		dst.Verbosity = loaded.Verbosity
	}
	return nil
}

func loadGlobalIgnores() gitignore.IgnoreMatcher {
	var lines []string
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	candidates := []string{filepath.Join(configDir, "lx", "ignore")}

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
