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

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
type OpenAIProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewOpenAIProvider creates an OpenAI provider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL:    "https://api.openai.com/v1",
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// NewOpenAICompatibleProvider creates a provider for OpenAI-compatible APIs (OpenRouter, local servers, etc.)
func NewOpenAICompatibleProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

func (o *OpenAIProvider) ID() string { return "openai" }

func (o *OpenAIProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", o.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("openai: invalid API key")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("openai returned status %d", resp.StatusCode)
	}
	return nil
}

func (o *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(result.Data))
	for i, m := range result.Data {
		models[i] = ModelInfo{ID: m.ID, Name: m.ID, SupportsTools: true}
	}
	return models, nil
}

// openaiChatRequest is the request body for OpenAI chat completions.
type openaiChatRequest struct {
	Model       string           `json:"model"`
	Messages    []openaiMessage  `json:"messages"`
	Stream      bool             `json:"stream"`
	Tools       []openaiTool     `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	StreamOptions *openaiStreamOpts `json:"stream_options,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []openaiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function openaiToolFunction  `json:"function"`
}

type openaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolDef      `json:"function"`
}

type openaiToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// openaiStreamDeltaToolCall is a tool call delta in the streaming response.
type openaiStreamDeltaToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openaiToolFunction `json:"function"`
}

// openaiStreamChunk is a single SSE chunk from the streaming response.
type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string                      `json:"content"`
			ToolCalls []openaiStreamDeltaToolCall  `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (o *OpenAIProvider) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan StreamChunk, error) {
	oaiMessages := make([]openaiMessage, len(messages))
	for i, m := range messages {
		om := openaiMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openaiToolFunction{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		oaiMessages[i] = om
	}

	var oaiTools []openaiTool
	for _, t := range opts.Tools {
		oaiTools = append(oaiTools, openaiTool{
			Type: "function",
			Function: openaiToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters},
		})
	}

	reqBody := openaiChatRequest{
		Model:    opts.Model,
		Messages: oaiMessages,
		Stream:   true,
		Tools:    oaiTools,
		StreamOptions: &openaiStreamOpts{IncludeUsage: true},
	}
	if opts.MaxTokens > 0 {
		reqBody.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		reqBody.Temperature = &opts.Temperature
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("openai rate limited: %s", string(body))
		}
		return nil, fmt.Errorf("openai returned status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		// Accumulate tool calls across chunks (OpenAI streams them incrementally)
		pendingToolCalls := make(map[int]*ToolCall)

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
			if data == "[DONE]" {
				break
			}

			var chunk openaiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta

				// Text content
				if delta.Content != "" {
					ch <- StreamChunk{Text: delta.Content}
				}

				// Tool calls (streamed incrementally by index)
				for _, tc := range delta.ToolCalls {
					idx := tc.Index
					if existing, ok := pendingToolCalls[idx]; ok {
						existing.Arguments += tc.Function.Arguments
						if tc.Function.Name != "" {
							existing.Name = tc.Function.Name
						}
					} else {
						pendingToolCalls[idx] = &ToolCall{
							ID:        tc.ID,
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						}
					}
				}

				// Emit completed tool calls on finish_reason=tool_calls or stop
				fr := chunk.Choices[0].FinishReason
				if fr != nil && (*fr == "tool_calls" || *fr == "stop") {
					for _, tc := range pendingToolCalls {
						ch <- StreamChunk{ToolCall: tc}
					}
					pendingToolCalls = make(map[int]*ToolCall)
				}
			}

			// Handle usage (sent on final chunk when stream_options.include_usage is true)
			if chunk.Usage != nil {
				ch <- StreamChunk{
					Done: true,
					Usage: &Usage{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
						Model:        opts.Model,
					},
				}
				return
			}
		}
	}()

	return ch, nil
}
