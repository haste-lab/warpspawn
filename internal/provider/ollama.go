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
	"time"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewOllamaProvider creates an Ollama provider with the given base URL.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 0, // no timeout for streaming — per-request timeouts via context
		},
	}
}

// httpClientWithTimeout returns a client for non-streaming requests.
func httpClientWithTimeout() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func (o *OllamaProvider) ID() string { return "ollama" }

func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", o.BaseURL+"/api/version", nil)
	if err != nil {
		return fmt.Errorf("ollama health check: %w", err)
	}
	resp, err := httpClientWithTimeout().Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", o.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}

func (o *OllamaProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClientWithTimeout().Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = ModelInfo{
			ID:            m.Name,
			Name:          m.Name,
			SupportsTools: true, // assume true; validated at assignment time
		}
	}
	return models, nil
}

// ollamaChatRequest is the request body for Ollama's /api/chat endpoint.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

// ollamaChatResponse is a single streamed response chunk.
type ollamaChatResponse struct {
	Message          ollamaMessage `json:"message"`
	Done             bool          `json:"done"`
	EvalCount        int           `json:"eval_count"`
	PromptEvalCount  int           `json:"prompt_eval_count"`
}

func (o *OllamaProvider) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan StreamChunk, error) {
	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, m := range messages {
		om := ollamaMessage{Role: m.Role, Content: m.Content}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaFunctionCall{
					Name:      tc.Name,
					Arguments: json.RawMessage(tc.Arguments),
				},
			})
		}
		ollamaMessages[i] = om
	}

	// Convert tools
	var ollamaTools []ollamaTool
	for _, t := range opts.Tools {
		ollamaTools = append(ollamaTools, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	reqBody := ollamaChatRequest{
		Model:    opts.Model,
		Messages: ollamaMessages,
		Stream:   true,
		Tools:    ollamaTools,
	}

	// Always set num_ctx — Ollama defaults to 2048 if not specified,
	// which is far too small for agentic tool-use workloads.
	numCtx := opts.ContextSize
	if numCtx < 2048 {
		numCtx = 16384 // default 16K if not configured
	}
	reqBody.Options = &ollamaOptions{
		NumCtx: numCtx,
	}
	if opts.Temperature > 0 {
		reqBody.Options.Temperature = opts.Temperature
	}
	if opts.MaxTokens > 0 {
		reqBody.Options.NumPredict = opts.MaxTokens
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama chat request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		var totalInputTokens, totalOutputTokens int

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Error: ctx.Err(), Done: true}
				return
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk ollamaChatResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				ch <- StreamChunk{Error: fmt.Errorf("parse ollama response: %w", err), Done: true}
				return
			}

			// Accumulate token counts
			if chunk.PromptEvalCount > 0 {
				totalInputTokens = chunk.PromptEvalCount
			}
			if chunk.EvalCount > 0 {
				totalOutputTokens = chunk.EvalCount
			}

			// Handle tool calls
			if len(chunk.Message.ToolCalls) > 0 {
				for _, tc := range chunk.Message.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Function.Arguments)
					callID := tc.ID
					if callID == "" {
						callID = fmt.Sprintf("call_%d", time.Now().UnixNano())
					}
					ch <- StreamChunk{
						ToolCall: &ToolCall{
							ID:        callID,
							Name:      tc.Function.Name,
							Arguments: string(argsJSON),
						},
					}
				}
			}

			// Handle text content
			if chunk.Message.Content != "" {
				ch <- StreamChunk{Text: chunk.Message.Content}
			}

			// Handle completion
			if chunk.Done {
				ch <- StreamChunk{
					Done: true,
					Usage: &Usage{
						InputTokens:  totalInputTokens,
						OutputTokens: totalOutputTokens,
						Model:        opts.Model,
					},
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: err, Done: true}
		}
	}()

	return ch, nil
}
