package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider implements the Provider interface for the Claude API.
type AnthropicProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewAnthropicProvider creates an Anthropic provider.
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		BaseURL:    "https://api.anthropic.com",
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

func (a *AnthropicProvider) ID() string { return "anthropic" }

func (a *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Anthropic doesn't have a models list endpoint — send a minimal request
	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("anthropic: invalid API key")
	}
	// Any 2xx or even a 400 (bad model) means the API is reachable
	if resp.StatusCode >= 500 {
		return fmt.Errorf("anthropic server error: %d", resp.StatusCode)
	}
	return nil
}

func (a *AnthropicProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Anthropic doesn't have a public model list endpoint — return known models
	return []ModelInfo{
		{ID: "claude-opus-4-6", Name: "Claude Opus 4.6", SupportsTools: true},
		{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", SupportsTools: true},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", SupportsTools: true},
	}, nil
}

// anthropicRequest is the request body for Claude messages API.
type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system,omitempty"`
	Messages  []anthropicMessage  `json:"messages"`
	Stream    bool                `json:"stream"`
	Tools     []anthropicTool     `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string               `json:"role"`
	Content json.RawMessage      `json:"content"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicStreamEvent represents a single SSE event from the streaming response.
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block,omitempty"`
	Message *struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

func (a *AnthropicProvider) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan StreamChunk, error) {
	// Separate system message from conversation
	var system string
	var convMessages []Message
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			convMessages = append(convMessages, m)
		}
	}

	// Convert messages to Anthropic format
	anthMessages := make([]anthropicMessage, 0, len(convMessages))
	for _, m := range convMessages {
		var content json.RawMessage
		if m.Role == "tool" {
			// Tool results in Anthropic format
			block := []anthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}}
			content, _ = json.Marshal(block)
		} else if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// Assistant with tool calls
			var blocks []anthropicContentBlock
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: json.RawMessage(tc.Arguments),
				})
			}
			content, _ = json.Marshal(blocks)
		} else {
			content, _ = json.Marshal(m.Content)
		}

		anthMessages = append(anthMessages, anthropicMessage{
			Role:    m.Role,
			Content: content,
		})
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	var tools []anthropicTool
	for _, t := range opts.Tools {
		tools = append(tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	reqBody := anthropicRequest{
		Model:     opts.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  anthMessages,
		Stream:    true,
		Tools:     tools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("anthropic rate limited: %s", string(body))
		}
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		var currentToolCall *ToolCall
		var inputTokens, outputTokens int

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Error: ctx.Err(), Done: true}
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "message_start":
				if event.Message != nil {
					inputTokens = event.Message.Usage.InputTokens
				}

			case "content_block_start":
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					currentToolCall = &ToolCall{
						ID:   event.ContentBlock.ID,
						Name: event.ContentBlock.Name,
					}
				}

			case "content_block_delta":
				var delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
				}
				json.Unmarshal(event.Delta, &delta)

				if delta.Type == "text_delta" && delta.Text != "" {
					ch <- StreamChunk{Text: delta.Text}
				}
				if delta.Type == "input_json_delta" && currentToolCall != nil {
					currentToolCall.Arguments += delta.PartialJSON
				}

			case "content_block_stop":
				if currentToolCall != nil {
					ch <- StreamChunk{ToolCall: currentToolCall}
					currentToolCall = nil
				}

			case "message_delta":
				if event.Usage != nil {
					outputTokens = event.Usage.OutputTokens
				}

			case "message_stop":
				ch <- StreamChunk{
					Done: true,
					Usage: &Usage{
						InputTokens:  inputTokens,
						OutputTokens: outputTokens,
						Model:        opts.Model,
					},
				}
				return
			}
		}
	}()

	return ch, nil
}
