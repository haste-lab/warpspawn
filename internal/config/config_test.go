package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ConfigVersion != 1 {
		t.Errorf("version = %d, want 1", cfg.ConfigVersion)
	}
	if cfg.Execution.ShellMode != "restricted" {
		t.Errorf("shell mode = %q, want restricted", cfg.Execution.ShellMode)
	}
	if cfg.Execution.LLMContextSize != 16384 {
		t.Errorf("context size = %d, want 16384", cfg.Execution.LLMContextSize)
	}
	if cfg.Budget.DailyLimitUSD != 10.0 {
		t.Errorf("budget = %f, want 10.0", cfg.Budget.DailyLimitUSD)
	}
	if len(cfg.Roles) < 5 {
		t.Errorf("expected at least 5 roles, got %d", len(cfg.Roles))
	}
}

func TestValidateConfig_EnforcesBounds(t *testing.T) {
	cfg := Config{
		Execution: ExecutionConfig{
			MaxToolCalls:   0,
			AgentTimeoutS:  5,
			ShellMode:      "invalid",
			LLMContextSize: 1024,
		},
		Budget: BudgetConfig{DailyLimitUSD: -10},
	}
	validated := ValidateConfig(cfg)

	if validated.Execution.MaxToolCalls != 30 {
		t.Errorf("max tools = %d, want 30", validated.Execution.MaxToolCalls)
	}
	if validated.Execution.AgentTimeoutS != 240 {
		t.Errorf("timeout = %d, want 240", validated.Execution.AgentTimeoutS)
	}
	if validated.Execution.ShellMode != "restricted" {
		t.Errorf("shell mode = %q, want restricted", validated.Execution.ShellMode)
	}
	if validated.Execution.LLMContextSize != 16384 {
		t.Errorf("context = %d, want 16384", validated.Execution.LLMContextSize)
	}
	if validated.Budget.DailyLimitUSD != 0 {
		t.Errorf("budget = %f, want 0", validated.Budget.DailyLimitUSD)
	}
}

func TestValidateConfig_UpperBounds(t *testing.T) {
	cfg := Config{
		Execution: ExecutionConfig{
			MaxToolCalls:   999,
			AgentTimeoutS:  99999,
			LLMContextSize: 999999,
		},
		Budget: BudgetConfig{DailyLimitUSD: 99999},
	}
	validated := ValidateConfig(cfg)

	if validated.Execution.MaxToolCalls != 200 {
		t.Errorf("max tools = %d, want 200", validated.Execution.MaxToolCalls)
	}
	if validated.Execution.AgentTimeoutS != 3600 {
		t.Errorf("timeout = %d, want 3600", validated.Execution.AgentTimeoutS)
	}
	if validated.Execution.LLMContextSize != 131072 {
		t.Errorf("context = %d, want 131072", validated.Execution.LLMContextSize)
	}
	if validated.Budget.DailyLimitUSD != 1000 {
		t.Errorf("budget = %f, want 1000", validated.Budget.DailyLimitUSD)
	}
}

func TestLoadSaveConfig(t *testing.T) {
	dir := t.TempDir()

	// First load creates default
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if cfg.ConfigVersion != 1 {
		t.Error("expected default config")
	}

	// Modify and save
	cfg.Budget.DailyLimitUSD = 25.0
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload
	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg2.Budget.DailyLimitUSD != 25.0 {
		t.Errorf("budget = %f, want 25.0", cfg2.Budget.DailyLimitUSD)
	}
}

func TestLoadCorruptConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte("not json{{{"), 0600)

	cfg, err := Load(dir)
	if err == nil {
		t.Error("expected error for corrupt config")
	}
	// Should return defaults despite corruption
	if cfg.ConfigVersion != 1 {
		t.Error("expected default config on corruption")
	}
	// Backup should exist
	if _, err := os.Stat(configPath + ".corrupt"); os.IsNotExist(err) {
		t.Error("expected corrupt backup file")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	t1 := GenerateSessionToken()
	t2 := GenerateSessionToken()
	if len(t1) != 64 { // 32 bytes hex-encoded
		t.Errorf("token length = %d, want 64", len(t1))
	}
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
}

func TestDefaultPaths(t *testing.T) {
	paths := DefaultPaths()
	if paths.ConfigDir == "" || paths.DataDir == "" || paths.ProjectDir == "" {
		t.Error("paths should not be empty")
	}
}

func TestSaveAtomicPermissions(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Should be 0600 (owner-only)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Execution.LLMContextSize != 16384 {
		t.Errorf("context size lost in roundtrip: %d", decoded.Execution.LLMContextSize)
	}
}
