package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleAdapterInvoke(t *testing.T) {
	var authorization string
	var requestedModel string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		var payload openAIChatRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = payload.Model
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Hello from Nomici"}}],
			"usage": {"prompt_tokens": 4, "completion_tokens": 3, "total_tokens": 7}
		}`))
	}))
	defer server.Close()

	result, err := NewOpenAICompatibleAdapter().Invoke(context.Background(), server.URL, "test-model", "secret-key", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.Messages[0].Content != "Hello from Nomici" {
		t.Fatalf("unexpected content %q", result.Messages[0].Content)
	}
	if result.Usage.InputTokens != 4 || result.Usage.OutputTokens != 3 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if authorization != "Bearer secret-key" {
		t.Fatalf("expected bearer auth, got %q", authorization)
	}
	if requestedModel != "test-model" {
		t.Fatalf("expected model in request body, got %q", requestedModel)
	}
}

func TestOpenAICompatibleAdapterAuthFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()

	result, err := NewOpenAICompatibleAdapter().Invoke(context.Background(), server.URL, "test-model", "secret-key", InvokeRequest{})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed || result.Error.Code != ErrorAuthFailed {
		t.Fatalf("expected auth failure, got %+v", result)
	}
	if strings.Contains(result.Error.Message, "secret-key") {
		t.Fatal("error leaked api key")
	}
}

func TestOpenAICompatibleAdapterServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "error", http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := NewOpenAICompatibleAdapter().Invoke(context.Background(), server.URL, "test-model", "secret-key", InvokeRequest{})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed || result.Error.Code != ErrorEndpointUnavailable {
		t.Fatalf("expected endpoint unavailable, got %+v", result)
	}
}

func TestOpenAICompatibleAdapterInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`not-json`))
	}))
	defer server.Close()

	result, err := NewOpenAICompatibleAdapter().Invoke(context.Background(), server.URL, "test-model", "", InvokeRequest{})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed || result.Error.Code != ErrorInvalidResponse {
		t.Fatalf("expected invalid response, got %+v", result)
	}
}
