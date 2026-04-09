package provider

import "context"

// Message represents a chat message in the LLM conversation.
type Message struct {
	Role       string      `json:"role"`    // system, user, assistant, tool
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDef defines a tool the LLM can call.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// StreamChunk represents a piece of streaming output.
type StreamChunk struct {
	// Text content (incremental)
	Text string

	// Tool call (when the LLM wants to invoke a tool)
	ToolCall *ToolCall

	// Done signals the stream is complete
	Done bool

	// Usage is populated on the final chunk
	Usage *Usage

	// Error if something went wrong
	Error error
}

// Usage reports token consumption for a single API call.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Model        string `json:"model"`
}

// CompletionOptions configures a single LLM call.
type CompletionOptions struct {
	Model       string
	MaxTokens   int
	Temperature float64
	Tools       []ToolDef
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SupportsTools bool  `json:"supports_tools"`
}

// Provider is the interface all LLM providers implement.
type Provider interface {
	// ID returns the provider identifier (e.g., "ollama", "openai", "anthropic").
	ID() string

	// Complete sends messages to the LLM and returns a streaming channel.
	// The channel receives StreamChunks until Done or Error.
	// Cancel the context to abort.
	Complete(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan StreamChunk, error)

	// ListModels returns available models from this provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// HealthCheck verifies the provider is reachable and functional.
	HealthCheck(ctx context.Context) error
}
