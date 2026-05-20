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
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        8,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     10 * time.Second,
			},
		},
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
	if len(request.Tools) > 0 {
		payload.Tools = openAITools(request.Tools)
		payload.ToolChoice = "auto"
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
	if transport, ok := httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
		defer transport.CloseIdleConnections()
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
		if len(request.Tools) > 0 && retryableToolSchemaStatus(response.StatusCode) {
			retry := request
			retry.Tools = nil
			return adapter.Invoke(ctx, baseURL, model, apiKey, retry)
		}
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
		message := decoded.Choices[0].Message
		result.Messages = []Message{{Role: message.Role, Content: message.Content}}
		result.ToolCalls = openAIToolCalls(message.ToolCalls)
	}
	return result, nil
}

func retryableToolSchemaStatus(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusUnprocessableEntity
}

func chatCompletionsURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

type openAIChatRequest struct {
	Model      string       `json:"model"`
	Messages   []Message    `json:"messages"`
	Stream     bool         `json:"stream"`
	Tools      []openAITool `json:"tools,omitempty"`
	ToolChoice string       `json:"tool_choice,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func openAITools(tools []ToolSchema) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.ID) == "" {
			continue
		}
		parameters := tool.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result = append(result, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        tool.ID,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}
	return result
}

func openAIToolCalls(calls []openAIToolCall) []ToolCall {
	result := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			continue
		}
		input := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &input)
		}
		result = append(result, ToolCall{ID: call.ID, ToolID: name, Input: input})
	}
	return result
}
