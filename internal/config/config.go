package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Display DisplayConfig
	LLM     LLMConfig
}

type DisplayConfig struct {
	MaxMessageLines int `toml:"max_message_lines"`
}

type LLMConfig struct {
	Endpoint string `toml:"endpoint"`
	APIKey   string `toml:"api_key"`
	Model    string `toml:"model"`
	Enabled  bool   `toml:"enabled"`
	TimeoutS int    `toml:"timeout_seconds"`
}

var defaults = Config{
	Display: DisplayConfig{
		MaxMessageLines: 20,
	},
	LLM: LLMConfig{
		Model:    "nemotron3-nano",
		TimeoutS: 30,
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
		LLM *struct {
			Endpoint *string `toml:"endpoint"`
			APIKey   *string `toml:"api_key"`
			Model    *string `toml:"model"`
			Enabled  *bool   `toml:"enabled"`
			TimeoutS *int    `toml:"timeout_seconds"`
		}
	}
	if _, err := toml.DecodeFile(path, &fileCfg); err != nil {
		return
	}
	if fileCfg.Display != nil && fileCfg.Display.MaxMessageLines != nil {
		cfg.Display.MaxMessageLines = *fileCfg.Display.MaxMessageLines
	}
	if fileCfg.LLM != nil {
		if fileCfg.LLM.Endpoint != nil {
			cfg.LLM.Endpoint = *fileCfg.LLM.Endpoint
		}
		if fileCfg.LLM.APIKey != nil {
			cfg.LLM.APIKey = *fileCfg.LLM.APIKey
		}
		if fileCfg.LLM.Model != nil {
			cfg.LLM.Model = *fileCfg.LLM.Model
		}
		if fileCfg.LLM.Enabled != nil {
			cfg.LLM.Enabled = *fileCfg.LLM.Enabled
		}
		if fileCfg.LLM.TimeoutS != nil {
			cfg.LLM.TimeoutS = *fileCfg.LLM.TimeoutS
		}
	}
}

func mergeFromEnv(cfg *Config) {
	if v := os.Getenv("GC_MAX_MESSAGE_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Display.MaxMessageLines = n
		}
	}
	if v := os.Getenv("GC_LLM_ENDPOINT"); v != "" {
		cfg.LLM.Endpoint = v
	}
	if v := os.Getenv("GC_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("GC_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("GC_LLM_ENABLED"); v != "" {
		cfg.LLM.Enabled = v == "true" || v == "1"
	}
}
