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

type AnthropicAdapter struct {
	httpClient *http.Client
}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{httpClient: &http.Client{Timeout: 120 * time.Second}}
}

func (adapter *AnthropicAdapter) Invoke(ctx context.Context, baseURL string, model string, apiKey string, request InvokeRequest) (*InvokeResult, error) {
	timeout := 120 * time.Second
	if request.Options.TimeoutMs > 0 {
		timeout = time.Duration(request.Options.TimeoutMs) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := anthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		Messages:  anthropicMessages(request.Messages),
	}
	if len(request.Tools) > 0 {
		payload.Tools = anthropicTools(request.Tools)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Anthropic request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Anthropic request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("anthropic-version", "2023-06-01")
	if apiKey != "" {
		httpRequest.Header.Set("x-api-key", apiKey)
	}
	httpClient := adapter.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return &InvokeResult{Status: StatusFailed, Error: &AdapterError{Code: ErrorTimeout, Message: "request timed out", Retryable: true}}, nil
		}
		return &InvokeResult{Status: StatusFailed, Error: &AdapterError{Code: ErrorEndpointUnavailable, Message: "endpoint did not respond", Retryable: true}}, nil
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Anthropic response: %w", err)
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
			Error:  &AdapterError{Code: code, Message: fmt.Sprintf("provider returned HTTP %d", response.StatusCode), Retryable: retryable},
			RawRef: string(responseBody),
		}, nil
	}
	var decoded anthropicResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return &InvokeResult{Status: StatusFailed, Error: &AdapterError{Code: ErrorInvalidResponse, Message: "provider returned invalid JSON", Retryable: false}}, nil
	}
	content := strings.TrimSpace(decodedText(decoded.Content))
	return &InvokeResult{
		Status:    StatusCompleted,
		Messages:  []Message{{Role: "assistant", Content: content}},
		ToolCalls: anthropicToolCalls(decoded.Content),
		Usage: &UsageInfo{
			InputTokens:  decoded.Usage.InputTokens,
			OutputTokens: decoded.Usage.OutputTokens,
		},
	}, nil
}

func anthropicMessages(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		if role != "assistant" {
			role = "user"
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		result = append(result, Message{Role: role, Content: message.Content})
	}
	if len(result) == 0 {
		result = []Message{{Role: "user", Content: ""}}
	}
	return result
}

func decodedText(content []anthropicContent) string {
	var builder strings.Builder
	for _, block := range content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(block.Text)
		}
	}
	return builder.String()
}

type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []Message       `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func anthropicTools(tools []ToolSchema) []anthropicTool {
	result := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.ID)
		if name == "" {
			continue
		}
		schema := tool.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result = append(result, anthropicTool{Name: name, Description: tool.Description, InputSchema: schema})
	}
	return result
}

func anthropicToolCalls(content []anthropicContent) []ToolCall {
	result := []ToolCall{}
	for _, block := range content {
		if block.Type != "tool_use" || strings.TrimSpace(block.Name) == "" {
			continue
		}
		input := block.Input
		if input == nil {
			input = map[string]any{}
		}
		result = append(result, ToolCall{ID: block.ID, ToolID: block.Name, Input: input})
	}
	return result
}
