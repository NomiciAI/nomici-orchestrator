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

	body, err := json.Marshal(geminiRequest{Contents: geminiContents(request.Messages)})
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
		Status:   StatusCompleted,
		Messages: []Message{{Role: "assistant", Content: strings.TrimSpace(geminiText(decoded))}},
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
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
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
