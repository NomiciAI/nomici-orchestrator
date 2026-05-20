package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicAdapterInvoke(t *testing.T) {
	var toolCount int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-api-key") != "secret" {
			t.Fatalf("expected x-api-key header")
		}
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		var payload anthropicRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		toolCount = len(payload.Tools)
		_, _ = response.Write([]byte(`{"content":[{"type":"text","text":"hello anthropic"}],"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer server.Close()

	result, err := NewAnthropicAdapter().Invoke(context.Background(), server.URL, "claude-test", "secret", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []ToolSchema{{ID: "read_file", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted || result.Messages[0].Content != "hello anthropic" || result.Usage.OutputTokens != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if toolCount != 1 {
		t.Fatalf("expected tool schema in request, got %d", toolCount)
	}
}

func TestGeminiAdapterInvoke(t *testing.T) {
	var toolCount int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.RawQuery, "key=secret") {
			t.Fatalf("expected key query, got %s", request.URL.RawQuery)
		}
		if request.URL.Path != "/models/gemini-test:generateContent" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		var payload geminiRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload.Tools) > 0 {
			toolCount = len(payload.Tools[0].FunctionDeclarations)
		}
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello gemini"}]}}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":4}}`))
	}))
	defer server.Close()

	result, err := NewGeminiAdapter().Invoke(context.Background(), server.URL, "gemini-test", "secret", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []ToolSchema{{ID: "search", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted || result.Messages[0].Content != "hello gemini" || result.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if toolCount != 1 {
		t.Fatalf("expected tool schema in request, got %d", toolCount)
	}
}

func TestAnthropicAndGeminiToolCallDecoding(t *testing.T) {
	anthropicCalls := anthropicToolCalls([]anthropicContent{{
		Type:  "tool_use",
		ID:    "call_1",
		Name:  "read_file",
		Input: map[string]any{"path": "README.md"},
	}})
	if len(anthropicCalls) != 1 || anthropicCalls[0].ToolID != "read_file" {
		t.Fatalf("unexpected anthropic tool calls: %+v", anthropicCalls)
	}
	geminiCalls := geminiToolCalls(geminiResponse{Candidates: []struct {
		Content geminiContent `json:"content"`
	}{{
		Content: geminiContent{Parts: []geminiPart{{FunctionCall: &geminiFunctionCall{Name: "search", Args: map[string]any{"query": "Nomici"}}}}},
	}}})
	if len(geminiCalls) != 1 || geminiCalls[0].ToolID != "search" {
		t.Fatalf("unexpected gemini tool calls: %+v", geminiCalls)
	}
}
