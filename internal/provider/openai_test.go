package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" && r.Header.Get("Authorization") == "Bearer test-key" {
			fmt.Fprintf(w, `{"data":[{"id":"gpt-4"}]}`)
			return
		}
		w.WriteHeader(401)
	}))
	defer server.Close()

	p := NewOpenAICompatibleProvider(server.URL+"/v1", "test-key")
	err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestOpenAIHealthCheckBadKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	p := NewOpenAICompatibleProvider(server.URL+"/v1", "bad-key")
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestOpenAIListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"}]}`)
	}))
	defer server.Close()

	p := NewOpenAICompatibleProvider(server.URL+"/v1", "key")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestOpenAICompleteStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
		}
		for _, chunk := range chunks {
			fmt.Fprintln(w, chunk)
			fmt.Fprintln(w)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewOpenAICompatibleProvider(server.URL+"/v1", "key")
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Say hello"},
	}, CompletionOptions{Model: "test"})
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
	if usage.InputTokens != 10 || usage.OutputTokens != 5 {
		t.Errorf("usage = %d/%d, want 10/5", usage.InputTokens, usage.OutputTokens)
	}
}

func TestOpenAICompleteWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		fr := "tool_calls"
		chunks := []string{
			`data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"id":"","type":"function","function":{"name":"","arguments":"\"main.go\"}"}}]},"finish_reason":null}]}`,
		}
		for _, chunk := range chunks {
			fmt.Fprintln(w, chunk)
			fmt.Fprintln(w)
			flusher.Flush()
		}
		// Final chunk with finish_reason
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"%s\"}],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":8}}\n\n", fr)
		flusher.Flush()
	}))
	defer server.Close()

	p := NewOpenAICompatibleProvider(server.URL+"/v1", "key")
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Read main.go"},
	}, CompletionOptions{
		Model: "test",
		Tools: []ToolDef{{Name: "read_file", Description: "Read a file"}},
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
		}
	}
	if !gotToolCall {
		t.Error("expected a tool call")
	}
}
