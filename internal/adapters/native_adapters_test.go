package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicAdapterInvoke(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-api-key") != "secret" {
			t.Fatalf("expected x-api-key header")
		}
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"content":[{"type":"text","text":"hello anthropic"}],"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer server.Close()

	result, err := NewAnthropicAdapter().Invoke(context.Background(), server.URL, "claude-test", "secret", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted || result.Messages[0].Content != "hello anthropic" || result.Usage.OutputTokens != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGeminiAdapterInvoke(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.RawQuery, "key=secret") {
			t.Fatalf("expected key query, got %s", request.URL.RawQuery)
		}
		if request.URL.Path != "/models/gemini-test:generateContent" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello gemini"}]}}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":4}}`))
	}))
	defer server.Close()

	result, err := NewGeminiAdapter().Invoke(context.Background(), server.URL, "gemini-test", "secret", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted || result.Messages[0].Content != "hello gemini" || result.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
