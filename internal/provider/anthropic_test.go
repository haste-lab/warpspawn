package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "test-key" {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"id":"msg_test","type":"message","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":1}}`)
			return
		}
		w.WriteHeader(401)
	}))
	defer server.Close()

	p := &AnthropicProvider{BaseURL: server.URL, APIKey: "test-key", HTTPClient: &http.Client{}}
	err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestAnthropicHealthCheckBadKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	p := &AnthropicProvider{BaseURL: server.URL, APIKey: "bad", HTTPClient: &http.Client{}}
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAnthropicListModels(t *testing.T) {
	p := NewAnthropicProvider("key")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) < 3 {
		t.Errorf("expected at least 3 models, got %d", len(models))
	}
}

func TestAnthropicCompleteStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"usage":{"input_tokens":12,"output_tokens":0}}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","usage":{"output_tokens":7}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}
		for _, line := range events {
			fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := &AnthropicProvider{BaseURL: server.URL, APIKey: "key", HTTPClient: &http.Client{}}
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Say hello"},
	}, CompletionOptions{Model: "claude-haiku-4-5-20251001"})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	var fullText string
	var usage *Usage
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		fullText += chunk.Text
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}

	if fullText != "Hello world" {
		t.Errorf("text = %q, want %q", fullText, "Hello world")
	}
	if usage == nil {
		t.Fatal("no usage")
	}
	if usage.InputTokens != 12 {
		t.Errorf("input tokens = %d, want 12", usage.InputTokens)
	}
	if usage.OutputTokens != 7 {
		t.Errorf("output tokens = %d, want 7", usage.OutputTokens)
	}
}

func TestAnthropicCompleteWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"usage":{"input_tokens":15,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"read_file"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"main.go\"}"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","usage":{"output_tokens":10}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}
		for _, line := range events {
			fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := &AnthropicProvider{BaseURL: server.URL, APIKey: "key", HTTPClient: &http.Client{}}
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Read main.go"},
	}, CompletionOptions{
		Model: "claude-haiku-4-5-20251001",
		Tools: []ToolDef{{Name: "read_file", Description: "Read a file", Parameters: map[string]interface{}{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	var gotToolCall bool
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		if chunk.ToolCall != nil {
			gotToolCall = true
			if chunk.ToolCall.Name != "read_file" {
				t.Errorf("tool name = %q, want read_file", chunk.ToolCall.Name)
			}
			if chunk.ToolCall.ID != "toolu_01" {
				t.Errorf("tool id = %q, want toolu_01", chunk.ToolCall.ID)
			}
		}
	}
	if !gotToolCall {
		t.Error("expected a tool call")
	}
}
