package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/haste-lab/warpspawn/internal/provider"
)

// extractToolCallsFromText parses tool calls that models output as JSON text
// rather than through the native tool-use protocol.
func extractToolCallsFromText(text string) []provider.ToolCall {
	var calls []provider.ToolCall
	callNum := 0

	// Find all JSON objects that look like tool calls
	for _, block := range findJSONBlocks(text) {
		var parsed struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(block), &parsed); err != nil {
			continue
		}
		if parsed.Name == "" {
			continue
		}

		callNum++
		calls = append(calls, provider.ToolCall{
			ID:        fmt.Sprintf("text_call_%d", callNum),
			Name:      parsed.Name,
			Arguments: string(parsed.Arguments),
		})
	}
	return calls
}

// findJSONBlocks extracts top-level JSON object blocks from text.
func findJSONBlocks(text string) []string {
	var blocks []string
	i := 0
	for i < len(text) {
		// Find opening brace
		start := strings.IndexByte(text[i:], '{')
		if start < 0 {
			break
		}
		start += i

		// Find matching closing brace
		depth := 0
		end := -1
		inString := false
		escape := false
		for j := start; j < len(text); j++ {
			if escape {
				escape = false
				continue
			}
			ch := text[j]
			if ch == '\\' && inString {
				escape = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = j + 1
					break
				}
			}
		}
		if end < 0 {
			break
		}
		blocks = append(blocks, text[start:end])
		i = end
	}
	return blocks
}

// RunConfig configures an agent run.
type RunConfig struct {
	ProjectRoot    string
	Model          string
	Provider       provider.Provider
	SystemPrompt   string
	UserPrompt     string
	MaxToolCalls   int
	CommandTimeout time.Duration
	AgentTimeout   time.Duration // wallclock timeout for the entire run
	ShellMode      ShellMode
	MaxPromptLen   int // max characters in user prompt (for small context models)
	OnChunk        func(StreamEvent) // callback for streaming events
}

// StreamEvent is emitted during agent execution.
type StreamEvent struct {
	Type       string // "text", "tool_call", "tool_result", "complete", "error"
	Text       string
	ToolCall   *provider.ToolCall
	ToolResult *ToolResult
	Summary    string
	Usage      *provider.Usage
	Error      error
}

// RunResult is the outcome of an agent run.
type RunResult struct {
	Success      bool
	Summary      string
	ToolCalls    int
	TotalUsage   provider.Usage
	Error        error
}

// Run executes the agent tool loop: prompt → LLM → tool calls → execute → feed back → repeat.
func Run(ctx context.Context, cfg RunConfig) RunResult {
	if cfg.MaxToolCalls <= 0 {
		cfg.MaxToolCalls = 30
	}
	if cfg.CommandTimeout <= 0 {
		cfg.CommandTimeout = 30 * time.Second
	}
	if cfg.ShellMode == "" {
		cfg.ShellMode = ShellRestricted // default to restricted, not unrestricted
	}

	// Apply wallclock timeout for the entire agent run
	if cfg.AgentTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.AgentTimeout)
		defer cancel()
	}

	emit := cfg.OnChunk
	if emit == nil {
		emit = func(StreamEvent) {}
	}

	// Truncate prompt if model has small context window
	userPrompt := cfg.UserPrompt
	if cfg.MaxPromptLen > 0 && len(userPrompt) > cfg.MaxPromptLen {
		userPrompt = userPrompt[:cfg.MaxPromptLen] + "\n\n(prompt truncated to fit context window)"
		slog.Warn("prompt truncated for small context model", "original_len", len(cfg.UserPrompt), "max_len", cfg.MaxPromptLen)
	}

	messages := []provider.Message{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Build tool execution config
	toolCfg := ExecuteConfig{
		ProjectRoot:    cfg.ProjectRoot,
		CommandTimeout: cfg.CommandTimeout,
		ShellMode:      cfg.ShellMode,
	}

	tools := BuiltinTools()
	totalUsage := provider.Usage{Model: cfg.Model}
	toolCallCount := 0

	for {
		// Check tool call budget
		if toolCallCount >= cfg.MaxToolCalls {
			emit(StreamEvent{Type: "error", Error: fmt.Errorf("max tool calls (%d) exceeded", cfg.MaxToolCalls)})
			return RunResult{
				Success:    false,
				Summary:    fmt.Sprintf("Aborted: exceeded %d tool calls", cfg.MaxToolCalls),
				ToolCalls:  toolCallCount,
				TotalUsage: totalUsage,
				Error:      fmt.Errorf("max tool calls exceeded"),
			}
		}

		// Call LLM
		stream, err := cfg.Provider.Complete(ctx, messages, provider.CompletionOptions{
			Model: cfg.Model,
			Tools: tools,
		})
		if err != nil {
			emit(StreamEvent{Type: "error", Error: err})
			return RunResult{
				Success:    false,
				Summary:    fmt.Sprintf("LLM call failed: %v", err),
				ToolCalls:  toolCallCount,
				TotalUsage: totalUsage,
				Error:      err,
			}
		}

		// Collect response
		var responseText string
		var responseToolCalls []provider.ToolCall

		for chunk := range stream {
			if chunk.Error != nil {
				emit(StreamEvent{Type: "error", Error: chunk.Error})
				return RunResult{
					Success:    false,
					Summary:    fmt.Sprintf("Stream error: %v", chunk.Error),
					ToolCalls:  toolCallCount,
					TotalUsage: totalUsage,
					Error:      chunk.Error,
				}
			}

			if chunk.Text != "" {
				responseText += chunk.Text
				emit(StreamEvent{Type: "text", Text: chunk.Text})
			}

			if chunk.ToolCall != nil {
				responseToolCalls = append(responseToolCalls, *chunk.ToolCall)
				emit(StreamEvent{Type: "tool_call", ToolCall: chunk.ToolCall})
			}

			if chunk.Usage != nil {
				totalUsage.InputTokens += chunk.Usage.InputTokens
				totalUsage.OutputTokens += chunk.Usage.OutputTokens
			}
		}

		// Fallback: if no native tool calls but the text contains JSON tool calls, extract them.
		// This handles models that output tool calls as text rather than native format.
		if len(responseToolCalls) == 0 && len(responseText) > 0 {
			extracted := extractToolCallsFromText(responseText)
			if len(extracted) > 0 {
				slog.Debug("extracted tool calls from text", "count", len(extracted))
				responseToolCalls = extracted
				for i := range extracted {
					emit(StreamEvent{Type: "tool_call", ToolCall: &extracted[i]})
				}
			}
		}

		// Add assistant response to conversation
		assistantMsg := provider.Message{
			Role:      "assistant",
			Content:   responseText,
			ToolCalls: responseToolCalls,
		}
		messages = append(messages, assistantMsg)

		// If no tool calls, the agent is done
		if len(responseToolCalls) == 0 {
			emit(StreamEvent{Type: "complete", Summary: responseText})
			return RunResult{
				Success:    true,
				Summary:    responseText,
				ToolCalls:  toolCallCount,
				TotalUsage: totalUsage,
			}
		}

		// Execute each tool call
		for _, tc := range responseToolCalls {
			toolCallCount++

			// Check for task_complete signal
			if tc.Name == "task_complete" {
				result := ExecuteTool(toolCfg, tc)
				emit(StreamEvent{Type: "tool_result", ToolResult: &result})
				emit(StreamEvent{Type: "complete", Summary: result.Content})

				// Add tool result to messages for completeness
				messages = append(messages, provider.Message{
					Role:       "tool",
					Content:    result.Content,
					ToolCallID: tc.ID,
				})

				return RunResult{
					Success:    true,
					Summary:    result.Content,
					ToolCalls:  toolCallCount,
					TotalUsage: totalUsage,
				}
			}

			slog.Debug("executing tool", "name", tc.Name, "call_id", tc.ID, "count", toolCallCount)

			result := ExecuteTool(toolCfg, tc)
			emit(StreamEvent{Type: "tool_result", ToolResult: &result})

			if result.Error != nil {
				slog.Warn("tool execution error", "name", tc.Name, "error", result.Error)
			}

			// Feed result back to the LLM (truncate for small context models)
			content := result.Content
			maxResult := 8000 // default max result size
			if cfg.MaxPromptLen > 0 && cfg.MaxPromptLen < 8000 {
				maxResult = cfg.MaxPromptLen / 2 // for 4K context, limit results to ~2K
			}
			if len(content) > maxResult {
				content = content[:maxResult] + "\n... (truncated to fit context)"
			}

			messages = append(messages, provider.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			})
		}
	}
}
