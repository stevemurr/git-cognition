package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Display DisplayConfig
}

type DisplayConfig struct {
	MaxMessageLines int `toml:"max_message_lines"`
}

var defaults = Config{
	Display: DisplayConfig{
		MaxMessageLines: 20,
	},
}

func Load(gitDir string) (*Config, error) {
	cfg := defaults

	// Layer 1: global config
	globalPath, err := globalConfigPath()
	if err == nil {
		mergeFromFile(&cfg, globalPath)
	}

	// Layer 2: environment variables
	mergeFromEnv(&cfg)

	// Layer 3: per-repo config
	if gitDir != "" {
		repoPath := filepath.Join(gitDir, "gc-config")
		mergeFromFile(&cfg, repoPath)
	}

	return &cfg, nil
}

func globalConfigPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "git-cognition", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-cognition", "config.toml"), nil
}

func mergeFromFile(cfg *Config, path string) {
	var fileCfg struct {
		Display *struct {
			MaxMessageLines *int `toml:"max_message_lines"`
		}
	}
	if _, err := toml.DecodeFile(path, &fileCfg); err != nil {
		return
	}
	if fileCfg.Display != nil && fileCfg.Display.MaxMessageLines != nil {
		cfg.Display.MaxMessageLines = *fileCfg.Display.MaxMessageLines
	}
}

func mergeFromEnv(cfg *Config) {
	if v := os.Getenv("GC_MAX_MESSAGE_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Display.MaxMessageLines = n
		}
	}
}
