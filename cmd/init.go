package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initRepo bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up git-cognition hooks and aliases",
	Long:  "Global: installs Claude Code hooks and git aliases. With --repo: enables capture in the current repository.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initRepo, "repo", false, "enable capture for the current repository")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if initRepo {
		return runInitRepo()
	}
	return runInitGlobal()
}

func runInitGlobal() error {
	// 1. Config file
	configPath, err := configFilePath()
	if err != nil {
		return err
	}
	configCreated := false
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(configPath), 0o755)
		os.WriteFile(configPath, []byte("[display]\nmax_message_lines = 20\n"), 0o644)
		configCreated = true
	}

	// 2. Claude Code hooks
	settingsPath, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	hookUpdated, err := mergeClaudeHooks(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to update Claude hooks: %w", err)
	}

	// 3. Git aliases
	aliasUpdated := false
	for _, alias := range []struct{ name, cmd string }{
		{"why", "!git-cognition why"},
		{"session", "!git-cognition session"},
	} {
		out, _ := exec.Command("git", "config", "--global", "--get", "alias."+alias.name).Output()
		if strings.TrimSpace(string(out)) != alias.cmd {
			exec.Command("git", "config", "--global", "alias."+alias.name, alias.cmd).Run()
			aliasUpdated = true
		}
	}

	// Output
	fmt.Println(strings.Repeat("─", 50))
	status := func(label, path string, created bool) {
		state := "[exists]"
		if created {
			state = "[created]"
		}
		fmt.Fprintf(os.Stdout, "%-8s %s  %s\n", label, path, state)
	}
	status("Config:", configPath, configCreated)

	hookState := "[exists]"
	if hookUpdated {
		hookState = "[updated]"
	}
	fmt.Fprintf(os.Stdout, "%-8s %s  %s\n", "Hooks:", settingsPath, hookState)

	aliasState := "[exists]"
	if aliasUpdated {
		aliasState = "[updated]"
	}
	fmt.Fprintf(os.Stdout, "%-8s %s  %s\n", "Aliases:", "~/.gitconfig", aliasState)
	fmt.Println()
	fmt.Println("Enable capture in each repo:")
	fmt.Println("  cd your-project && git-cognition init --repo")
	fmt.Println(strings.Repeat("─", 50))

	return nil
}

func runInitRepo() error {
	// Init git repo if needed
	gitDir, err := findRepoGitDir()
	if err != nil {
		cmd := exec.Command("git", "init")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git init failed: %w", err)
		}
		gitDir, err = findRepoGitDir()
		if err != nil {
			return err
		}
	}

	// 1. Create gc-enabled marker
	enabledPath := filepath.Join(gitDir, "gc-enabled")
	enabledCreated := false
	if _, err := os.Stat(enabledPath); os.IsNotExist(err) {
		os.WriteFile(enabledPath, []byte(""), 0o644)
		enabledCreated = true
	}

	// 2. Add gc-sessions/ to .git/info/exclude
	excludePath := filepath.Join(gitDir, "info", "exclude")
	excludeUpdated := false
	os.MkdirAll(filepath.Join(gitDir, "info"), 0o755)
	content, _ := os.ReadFile(excludePath)
	if !strings.Contains(string(content), "gc-sessions/") {
		f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err == nil {
			f.WriteString("\ngc-sessions/\n")
			f.Close()
			excludeUpdated = true
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	status := func(label, path string, created bool) {
		state := "[exists]"
		if created {
			state = "[created]"
		}
		fmt.Fprintf(os.Stdout, "%-10s %s  %s\n", label, path, state)
	}
	status("Enabled:", enabledPath, enabledCreated)

	excludeState := "[exists]"
	if excludeUpdated {
		excludeState = "[updated]"
	}
	fmt.Fprintf(os.Stdout, "%-10s %s  %s\n", "Excluded:", excludePath, excludeState)
	fmt.Println(strings.Repeat("─", 50))

	return nil
}

func configFilePath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "git-cognition", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-cognition", "config.toml"), nil
}

func claudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func mergeClaudeHooks(path string) (bool, error) {
	// Read existing settings
	var settings map[string]interface{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
		} else {
			return false, err
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	// Backup before modifying
	if len(data) > 0 {
		os.WriteFile(path+".bak", data, 0o644)
	}

	// Desired hooks
	postToolUseHook := map[string]interface{}{
		"matcher": "",
		"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "git-cognition hook post-tool-use"}},
	}
	stopHook := map[string]interface{}{
		"hooks": []interface{}{map[string]interface{}{"type": "command", "command": "git-cognition hook stop"}},
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	updated := false

	// Merge PostToolUse
	if !containsHook(hooks, "PostToolUse", "git-cognition hook post-tool-use") {
		ptu, _ := hooks["PostToolUse"].([]interface{})
		hooks["PostToolUse"] = append(ptu, postToolUseHook)
		updated = true
	}

	// Merge Stop
	if !containsHook(hooks, "Stop", "git-cognition hook stop") {
		stop, _ := hooks["Stop"].([]interface{})
		hooks["Stop"] = append(stop, stopHook)
		updated = true
	}

	if !updated {
		return false, nil
	}

	settings["hooks"] = hooks
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	os.MkdirAll(filepath.Dir(path), 0o755)
	return true, os.WriteFile(path, out, 0o644)
}

func containsHook(hooks map[string]interface{}, event, command string) bool {
	arr, ok := hooks[event].([]interface{})
	if !ok {
		return false
	}
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		hookArr, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range hookArr {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if hm["command"] == command {
				return true
			}
		}
	}
	return false
}

func findRepoGitDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return gitPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}
