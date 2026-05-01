package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatibleAdapter struct {
	httpClient *http.Client
}

func NewOpenAICompatibleAdapter() *OpenAICompatibleAdapter {
	return &OpenAICompatibleAdapter{
		httpClient: &http.Client{},
	}
}

func (adapter *OpenAICompatibleAdapter) Invoke(ctx context.Context, baseURL string, model string, apiKey string, request InvokeRequest) (*InvokeResult, error) {
	timeout := 120 * time.Second
	if request.Options.TimeoutMs > 0 {
		timeout = time.Duration(request.Options.TimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := openAIChatRequest{
		Model:    model,
		Messages: request.Messages,
		Stream:   false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI-compatible request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI-compatible request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	}

	httpClient := adapter.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return &InvokeResult{
				Status: StatusFailed,
				Error:  &AdapterError{Code: ErrorTimeout, Message: "request timed out", Retryable: true},
			}, nil
		}
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorEndpointUnavailable, Message: "endpoint did not respond", Retryable: true},
		}, nil
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read OpenAI-compatible response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := ErrorEndpointUnavailable
		retryable := response.StatusCode >= 500
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			code = ErrorAuthFailed
			retryable = false
		}
		return &InvokeResult{
			Status: StatusFailed,
			Error: &AdapterError{
				Code:      code,
				Message:   fmt.Sprintf("provider returned HTTP %d", response.StatusCode),
				Retryable: retryable,
			},
			RawRef: string(responseBody),
		}, nil
	}

	var decoded openAIChatResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error: &AdapterError{
				Code:      ErrorInvalidResponse,
				Message:   "provider returned invalid JSON",
				Retryable: false,
			},
		}, nil
	}

	result := &InvokeResult{
		Status: StatusCompleted,
		Usage: &UsageInfo{
			InputTokens:  decoded.Usage.PromptTokens,
			OutputTokens: decoded.Usage.CompletionTokens,
		},
	}
	if len(decoded.Choices) > 0 {
		result.Messages = []Message{decoded.Choices[0].Message}
	}
	return result, nil
}

func chatCompletionsURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

type openAIChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
