package guard

import (
	"os"
	"testing"
)

func TestBudgetCheckNewDay(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(dir)

	check := b.Check()
	if !check.Allowed {
		t.Error("new budget should be allowed")
	}
	if check.UsedUSD != 0 {
		t.Errorf("expected 0 used, got %f", check.UsedUSD)
	}
	if check.LimitUSD != DefaultDailyLimitUSD {
		t.Errorf("expected default limit, got %f", check.LimitUSD)
	}
}

func TestBudgetRecord(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(dir)

	result := b.Record(BudgetEntry{
		Project:      "test-project",
		Role:         "builder",
		TaskID:       "TASK-001",
		Model:        "gpt-5.4-mini",
		InputTokens:  1000,
		OutputTokens: 500,
	})

	if result.UsedUSD == 0 {
		t.Error("expected non-zero cost")
	}
	if !result.Allowed {
		t.Error("should still be within budget")
	}

	// Verify persistence
	b2 := NewBudget(dir)
	check := b2.Check()
	if check.UsedUSD != result.UsedUSD {
		t.Errorf("persisted cost mismatch: %f vs %f", check.UsedUSD, result.UsedUSD)
	}
}

func TestBudgetExhaustion(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(dir)
	b.SetDailyLimit(0.01) // very low limit

	result := b.Record(BudgetEntry{
		Model:        "gpt-5.4",
		InputTokens:  10000,
		OutputTokens: 5000,
	})

	if result.Allowed {
		t.Error("should be over budget")
	}
}

func TestCalculateCost(t *testing.T) {
	// gpt-5.4-mini: $0.001/1K in, $0.003/1K out
	cost := CalculateCost("gpt-5.4-mini", 1000, 1000)
	expected := 0.001 + 0.003
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Errorf("cost = %f, want ~%f", cost, expected)
	}

	// Unknown model (Ollama, free)
	cost = CalculateCost("qwen2.5-coder:7b", 10000, 5000)
	if cost != 0 {
		t.Errorf("ollama cost should be 0, got %f", cost)
	}
}

func TestBudgetFileNotExist(t *testing.T) {
	dir := t.TempDir()
	os.Remove(dir) // remove the dir itself
	b := NewBudget(dir)
	check := b.Check()
	if !check.Allowed {
		t.Error("should be allowed even if dir doesn't exist")
	}
}
