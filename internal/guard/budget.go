package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const DefaultDailyLimitUSD = 10.0

// TokenCost maps model names to cost per 1K tokens (input, output).
var TokenCost = map[string][2]float64{
	// OpenAI
	"gpt-5.4":      {0.01, 0.03},
	"gpt-5.4-mini": {0.001, 0.003},
	// Anthropic
	"claude-sonnet-4-6":  {0.003, 0.015},
	"claude-haiku-4-5":   {0.0008, 0.004},
	// Ollama (free)
	"_default": {0, 0},
}

// BudgetEntry records a single LLM call.
type BudgetEntry struct {
	Timestamp    string  `json:"timestamp"`
	Project      string  `json:"project"`
	Role         string  `json:"role"`
	TaskID       string  `json:"task_id"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// BudgetState tracks daily token/cost usage.
type BudgetState struct {
	Date         string        `json:"date"`
	TotalCostUSD float64       `json:"total_cost_usd"`
	DailyLimitUSD float64      `json:"daily_limit_usd"`
	TotalInputTokens  int      `json:"total_input_tokens"`
	TotalOutputTokens int      `json:"total_output_tokens"`
	Entries      []BudgetEntry `json:"entries"`
}

// BudgetCheck is the result of checking budget availability.
type BudgetCheck struct {
	Allowed      bool    `json:"allowed"`
	RemainingUSD float64 `json:"remaining_usd"`
	UsedUSD      float64 `json:"used_usd"`
	LimitUSD     float64 `json:"limit_usd"`
	Date         string  `json:"date"`
}

// Budget manages token/cost tracking with file persistence.
type Budget struct {
	mu       sync.Mutex
	dataDir  string
	state    BudgetState
}

// NewBudget creates a budget tracker backed by a file in dataDir.
func NewBudget(dataDir string) *Budget {
	b := &Budget{dataDir: dataDir}
	b.load()
	return b
}

func (b *Budget) budgetPath() string {
	return filepath.Join(b.dataDir, "budget.json")
}

func (b *Budget) load() {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")

	data, err := os.ReadFile(b.budgetPath())
	if err != nil {
		b.state = BudgetState{Date: today, DailyLimitUSD: DefaultDailyLimitUSD}
		return
	}

	var state BudgetState
	if err := json.Unmarshal(data, &state); err != nil {
		b.state = BudgetState{Date: today, DailyLimitUSD: DefaultDailyLimitUSD}
		return
	}

	// Reset if new day
	if state.Date != today {
		b.state = BudgetState{Date: today, DailyLimitUSD: state.DailyLimitUSD}
		return
	}

	if state.DailyLimitUSD <= 0 {
		state.DailyLimitUSD = DefaultDailyLimitUSD
	}
	b.state = state
}

func (b *Budget) save() {
	os.MkdirAll(b.dataDir, 0755)
	data, _ := json.MarshalIndent(b.state, "", "  ")
	tmpPath := b.budgetPath() + ".tmp"
	os.WriteFile(tmpPath, append(data, '\n'), 0644)
	os.Rename(tmpPath, b.budgetPath())
}

// Check returns whether budget is available.
func (b *Budget) Check() BudgetCheck {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if b.state.Date != today {
		b.state = BudgetState{Date: today, DailyLimitUSD: b.state.DailyLimitUSD}
	}

	return BudgetCheck{
		Allowed:      b.state.TotalCostUSD < b.state.DailyLimitUSD,
		RemainingUSD: b.state.DailyLimitUSD - b.state.TotalCostUSD,
		UsedUSD:      b.state.TotalCostUSD,
		LimitUSD:     b.state.DailyLimitUSD,
		Date:         b.state.Date,
	}
}

// Record adds a token usage entry and persists.
func (b *Budget) Record(entry BudgetEntry) BudgetCheck {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if b.state.Date != today {
		b.state = BudgetState{Date: today, DailyLimitUSD: b.state.DailyLimitUSD}
	}

	// Calculate cost
	entry.CostUSD = CalculateCost(entry.Model, entry.InputTokens, entry.OutputTokens)
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)

	b.state.TotalCostUSD += entry.CostUSD
	b.state.TotalInputTokens += entry.InputTokens
	b.state.TotalOutputTokens += entry.OutputTokens
	b.state.Entries = append(b.state.Entries, entry)

	b.save()

	return BudgetCheck{
		Allowed:      b.state.TotalCostUSD < b.state.DailyLimitUSD,
		RemainingUSD: b.state.DailyLimitUSD - b.state.TotalCostUSD,
		UsedUSD:      b.state.TotalCostUSD,
		LimitUSD:     b.state.DailyLimitUSD,
		Date:         b.state.Date,
	}
}

// SetDailyLimit updates the daily cost limit.
func (b *Budget) SetDailyLimit(limitUSD float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state.DailyLimitUSD = limitUSD
	b.save()
}

// CalculateCost computes the USD cost for a given model and token counts.
func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	costs, ok := TokenCost[model]
	if !ok {
		costs = TokenCost["_default"]
	}
	inputCost := float64(inputTokens) / 1000.0 * costs[0]
	outputCost := float64(outputTokens) / 1000.0 * costs[1]
	return inputCost + outputCost
}
