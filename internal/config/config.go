package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all application configuration.
type Config struct {
	ConfigVersion int                       `json:"config_version"`
	Providers     map[string]ProviderConfig `json:"providers"`
	Roles         map[string]RoleConfig     `json:"roles"`
	Budget        BudgetConfig              `json:"budget"`
	Execution     ExecutionConfig           `json:"execution"`
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url,omitempty"`
	// API key is stored in keyring, not here. KeyRef points to the keyring entry.
	KeyRef  string `json:"key_ref,omitempty"`
}

// RoleConfig maps a role to its provider and model.
type RoleConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// BudgetConfig holds budget limits.
type BudgetConfig struct {
	DailyLimitUSD float64 `json:"daily_limit_usd"`
}

// ExecutionConfig holds execution behavior settings.
type ExecutionConfig struct {
	MaxToolCalls   int    `json:"max_tool_calls"`
	AgentTimeoutS  int    `json:"agent_timeout_s"`
	ShellMode      string `json:"shell_mode"` // unrestricted, restricted, approval
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		ConfigVersion: 1,
		Providers: map[string]ProviderConfig{
			"ollama":    {Enabled: true, BaseURL: "http://localhost:11434"},
			"openai":    {Enabled: false, KeyRef: "keyring:warpspawn/openai"},
			"anthropic": {Enabled: false, KeyRef: "keyring:warpspawn/anthropic"},
		},
		Roles: map[string]RoleConfig{
			"mission-control": {Provider: "ollama", Model: "qwen3:8b"},
			"architect":       {Provider: "ollama", Model: "qwen3:8b"},
			"ux":              {Provider: "ollama", Model: "qwen3:8b"},
			"builder":         {Provider: "ollama", Model: "qwen2.5-coder:7b"},
			"builder-light":   {Provider: "ollama", Model: "qwen2.5-coder:7b"},
			"reviewer-qa":     {Provider: "ollama", Model: "qwen3:8b"},
		},
		Budget: BudgetConfig{DailyLimitUSD: 10.0},
		Execution: ExecutionConfig{
			MaxToolCalls:  30,
			AgentTimeoutS: 240,
			ShellMode:     "restricted",
		},
	}
}

// Paths returns standard application paths.
type Paths struct {
	ConfigDir  string // ~/.config/warpspawn
	DataDir    string // ~/.local/share/warpspawn
	ProjectDir string // ~/.local/share/warpspawn/projects
}

// DefaultPaths returns XDG-compliant paths.
func DefaultPaths() Paths {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "warpspawn")
	dataDir := filepath.Join(home, ".local", "share", "warpspawn")
	return Paths{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		ProjectDir: filepath.Join(dataDir, "projects"),
	}
}

// Load reads config from disk, creating defaults if it doesn't exist.
func Load(configDir string) (Config, error) {
	configPath := filepath.Join(configDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			Save(configDir, cfg)
			return cfg, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Config file is corrupt — log warning, use defaults, backup the bad file
		backupPath := configPath + ".corrupt"
		os.WriteFile(backupPath, data, 0600)
		cfg = DefaultConfig()
		Save(configDir, cfg)
		return cfg, fmt.Errorf("config parse error (defaults restored, corrupt file saved to %s): %w", backupPath, err)
	}

	if cfg.ConfigVersion == 0 {
		cfg = DefaultConfig()
	}

	// Validate and enforce bounds
	cfg = ValidateConfig(cfg)
	return cfg, nil
}

// ValidateConfig enforces safe bounds on configuration values.
func ValidateConfig(cfg Config) Config {
	if cfg.Execution.MaxToolCalls < 1 {
		cfg.Execution.MaxToolCalls = 30
	}
	if cfg.Execution.MaxToolCalls > 200 {
		cfg.Execution.MaxToolCalls = 200
	}
	if cfg.Execution.AgentTimeoutS < 10 {
		cfg.Execution.AgentTimeoutS = 240
	}
	if cfg.Execution.AgentTimeoutS > 3600 {
		cfg.Execution.AgentTimeoutS = 3600
	}
	if cfg.Budget.DailyLimitUSD < 0 {
		cfg.Budget.DailyLimitUSD = 0
	}
	if cfg.Budget.DailyLimitUSD > 1000 {
		cfg.Budget.DailyLimitUSD = 1000
	}
	validModes := map[string]bool{"unrestricted": true, "restricted": true, "approval": true}
	if !validModes[cfg.Execution.ShellMode] {
		cfg.Execution.ShellMode = "restricted"
	}
	return cfg
}

// Save writes config to disk.
func Save(configDir string, cfg Config) error {
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, configPath)
}

// GenerateSessionToken creates a cryptographic token for localhost API auth.
func GenerateSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
