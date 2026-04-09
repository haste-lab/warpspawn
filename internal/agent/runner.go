package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/haste-lab/warpspawn/internal/provider"
)

// RunConfig configures an agent run.
type RunConfig struct {
	ProjectRoot    string
	Model          string
	Provider       provider.Provider
	SystemPrompt   string
	UserPrompt     string
	MaxToolCalls   int
	CommandTimeout time.Duration
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

	emit := cfg.OnChunk
	if emit == nil {
		emit = func(StreamEvent) {}
	}

	messages := []provider.Message{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: cfg.UserPrompt},
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
				result := ExecuteTool(cfg.ProjectRoot, tc, cfg.CommandTimeout)
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

			result := ExecuteTool(cfg.ProjectRoot, tc, cfg.CommandTimeout)
			emit(StreamEvent{Type: "tool_result", ToolResult: &result})

			if result.Error != nil {
				slog.Warn("tool execution error", "name", tc.Name, "error", result.Error)
			}

			// Feed result back to the LLM
			messages = append(messages, provider.Message{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: tc.ID,
			})
		}
	}
}
