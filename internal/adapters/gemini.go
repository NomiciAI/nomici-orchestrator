package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GeminiAdapter struct {
	httpClient *http.Client
}

func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{httpClient: &http.Client{Timeout: 120 * time.Second}}
}

func (adapter *GeminiAdapter) Invoke(ctx context.Context, baseURL string, model string, apiKey string, request InvokeRequest) (*InvokeResult, error) {
	timeout := 120 * time.Second
	if request.Options.TimeoutMs > 0 {
		timeout = time.Duration(request.Options.TimeoutMs) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := geminiRequest{Contents: geminiContents(request.Messages)}
	if len(request.Tools) > 0 {
		payload.Tools = geminiTools(request.Tools)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini request: %w", err)
	}
	model = strings.TrimPrefix(model, "models/")
	endpoint := strings.TrimRight(baseURL, "/") + "/models/" + url.PathEscape(model) + ":generateContent"
	if apiKey != "" {
		endpoint += "?key=" + url.QueryEscape(apiKey)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Gemini request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("read Gemini response: %w", err)
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
	var decoded geminiResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return &InvokeResult{Status: StatusFailed, Error: &AdapterError{Code: ErrorInvalidResponse, Message: "provider returned invalid JSON", Retryable: false}}, nil
	}
	return &InvokeResult{
		Status:    StatusCompleted,
		Messages:  []Message{{Role: "assistant", Content: strings.TrimSpace(geminiText(decoded))}},
		ToolCalls: geminiToolCalls(decoded),
		Usage: &UsageInfo{
			InputTokens:  decoded.UsageMetadata.PromptTokenCount,
			OutputTokens: decoded.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}

func geminiContents(messages []Message) []geminiContent {
	result := make([]geminiContent, 0, len(messages))
	for _, message := range messages {
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		role := "user"
		if message.Role == "assistant" || message.Role == "model" {
			role = "model"
		}
		result = append(result, geminiContent{Role: role, Parts: []geminiPart{{Text: text}}})
	}
	if len(result) == 0 {
		result = []geminiContent{{Role: "user", Parts: []geminiPart{{Text: ""}}}}
	}
	return result
}

func geminiText(response geminiResponse) string {
	if len(response.Candidates) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range response.Candidates[0].Content.Parts {
		if strings.TrimSpace(part.Text) != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
	Tools    []geminiTool    `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func geminiTools(tools []ToolSchema) []geminiTool {
	declarations := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.ID)
		if name == "" {
			continue
		}
		parameters := tool.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		declarations = append(declarations, geminiFunctionDeclaration{Name: name, Description: tool.Description, Parameters: parameters})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []geminiTool{{FunctionDeclarations: declarations}}
}

func geminiToolCalls(response geminiResponse) []ToolCall {
	if len(response.Candidates) == 0 {
		return nil
	}
	result := []ToolCall{}
	for _, part := range response.Candidates[0].Content.Parts {
		if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
			continue
		}
		input := part.FunctionCall.Args
		if input == nil {
			input = map[string]any{}
		}
		result = append(result, ToolCall{ToolID: part.FunctionCall.Name, Input: input})
	}
	return result
}
