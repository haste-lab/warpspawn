package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"version":"0.20.0"}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	err := p.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestOllamaHealthCheckUnreachable(t *testing.T) {
	p := NewOllamaProvider("http://localhost:1") // unlikely to be running
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestOllamaListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{
					{"name": "qwen2.5-coder:7b"},
					{"name": "qwen3:8b"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "qwen2.5-coder:7b" {
		t.Errorf("first model = %q", models[0].ID)
	}
}

func TestOllamaCompleteStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("server does not support flushing")
			}
			w.Header().Set("Content-Type", "application/x-ndjson")

			// Simulate streaming response
			chunks := []string{
				`{"message":{"role":"assistant","content":"Hello"},"done":false}`,
				`{"message":{"role":"assistant","content":" world"},"done":false}`,
				`{"message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":5}`,
			}
			for _, chunk := range chunks {
				fmt.Fprintln(w, chunk)
				flusher.Flush()
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Say hello"},
	}, CompletionOptions{Model: "test-model"})
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
		t.Errorf("full text = %q, want %q", fullText, "Hello world")
	}
	if usage == nil {
		t.Fatal("no usage data received")
	}
	if usage.InputTokens != 10 {
		t.Errorf("input tokens = %d, want 10", usage.InputTokens)
	}
	if usage.OutputTokens != 5 {
		t.Errorf("output tokens = %d, want 5", usage.OutputTokens)
	}
}

func TestOllamaCompleteWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			w.Header().Set("Content-Type", "application/x-ndjson")
			// Simulate a tool call response
			fmt.Fprintln(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"read_file","arguments":{"path":"main.go"}}}]},"done":false}`)
			fmt.Fprintln(w, `{"message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":15,"eval_count":8}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ch, err := p.Complete(context.Background(), []Message{
		{Role: "user", Content: "Read main.go"},
	}, CompletionOptions{
		Model: "test-model",
		Tools: []ToolDef{
			{Name: "read_file", Description: "Read a file", Parameters: map[string]interface{}{"type": "object"}},
		},
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

func TestOllamaCompleteContextCancel(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		// Send one chunk to establish the stream, then block
		fmt.Fprintln(w, `{"message":{"role":"assistant","content":"start"},"done":false}`)
		flusher.Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL)
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := p.Complete(ctx, []Message{
		{Role: "user", Content: "test"},
	}, CompletionOptions{Model: "test-model"})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	// Wait for first chunk to confirm stream is established
	<-started

	// Cancel the context
	cancel()

	// Drain — should terminate quickly
	for chunk := range ch {
		if chunk.Error != nil {
			return // expected: context cancelled
		}
	}
	// Channel closed — also acceptable
}
