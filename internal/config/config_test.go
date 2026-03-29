package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Display.MaxMessageLines != 20 {
		t.Errorf("max_message_lines = %d, want 20", cfg.Display.MaxMessageLines)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("GC_MAX_MESSAGE_LINES", "50")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Display.MaxMessageLines != 50 {
		t.Errorf("max_message_lines = %d, want 50", cfg.Display.MaxMessageLines)
	}
}

func TestFileOverride(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "git-cognition")
	os.MkdirAll(cfgDir, 0o755)
	cfgPath := filepath.Join(cfgDir, "config.toml")
	os.WriteFile(cfgPath, []byte("[display]\nmax_message_lines = 0\n"), 0o644)

	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Display.MaxMessageLines != 0 {
		t.Errorf("max_message_lines = %d, want 0", cfg.Display.MaxMessageLines)
	}
}

func TestRepoOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "gc-config"), []byte("[display]\nmax_message_lines = 10\n"), 0o644)

	// Use a nonexistent XDG path so global config doesn't interfere
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "no-such-dir"))

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Display.MaxMessageLines != 10 {
		t.Errorf("max_message_lines = %d, want 10", cfg.Display.MaxMessageLines)
	}
}
