package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

type v1ModelListResponse struct {
	Object string    `json:"object"`
	Data   []v1Model `json:"data"`
}

type v1Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type v1ChatCompletionRequest struct {
	Model    string             `json:"model"`
	Messages []adapters.Message `json:"messages"`
	Stream   bool               `json:"stream"`
}

type v1ChatCompletionResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []v1ChatCompletionChoice `json:"choices"`
	Usage   *v1ChatCompletionUsage   `json:"usage,omitempty"`
}

type v1ChatCompletionChoice struct {
	Index        int              `json:"index"`
	Message      adapters.Message `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type v1ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func v1ModelsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if services.Providers == nil {
			writeOpenAIError(response, http.StatusServiceUnavailable, "models_unavailable", "Model registry is not initialized.")
			return
		}
		profiles, err := services.Providers.List(request.Context())
		if err != nil {
			writeOpenAIError(response, http.StatusInternalServerError, "models_list_failed", "Model profiles could not be loaded.")
			return
		}
		models := make([]v1Model, 0, len(profiles))
		for _, profile := range profiles {
			models = append(models, v1Model{
				ID:      profile.ID,
				Object:  "model",
				Created: 0,
				OwnedBy: "nomici",
			})
		}
		writeJSON(response, http.StatusOK, v1ModelListResponse{Object: "list", Data: models})
	}
}

func v1ChatCompletionsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if services.Providers == nil || services.Secrets == nil || services.Adapter == nil || services.Trace == nil {
			writeOpenAIError(response, http.StatusServiceUnavailable, "gateway_unavailable", "Gateway model services are not initialized.")
			return
		}
		var body v1ChatCompletionRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeOpenAIError(response, http.StatusBadRequest, "invalid_request", "Request body must be JSON.")
			return
		}
		if body.Model == "" {
			writeOpenAIError(response, http.StatusBadRequest, "invalid_request", "model is required.")
			return
		}
		if len(body.Messages) == 0 {
			writeOpenAIError(response, http.StatusBadRequest, "invalid_request", "messages must contain at least one message.")
			return
		}
		if body.Stream {
			writeOpenAIError(response, http.StatusBadRequest, "stream_not_supported", "Streaming chat completions are not implemented in this Gateway slice.")
			return
		}

		ctx := request.Context()
		profile, err := services.Providers.Get(ctx, body.Model)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeOpenAIError(response, http.StatusNotFound, "model_not_found", "Configured model profile was not found.")
				return
			}
			writeOpenAIError(response, http.StatusInternalServerError, "model_load_failed", "Model profile could not be loaded.")
			return
		}

		var apiKey string
		var redactions []string
		if profile.APIKeyEnv != "" {
			resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
			if !ok {
				writeOpenAIError(response, http.StatusBadRequest, "missing_secret", "Provider API key environment variable is not set.")
				return
			}
			apiKey = resolved
			redactions = append(redactions, secrets.RedactedEnv(profile.APIKeyEnv))
		}

		runID := ids.New("run")
		requestID := newRequestID()
		if err := appendV1RunStarted(ctx, services.Trace, runID, requestID, profile.ID, profile.Model, body.Messages, redactions); err != nil {
			writeOpenAIError(response, http.StatusInternalServerError, "trace_failed", "Run trace could not be created.")
			return
		}

		result, err := services.Adapter.Invoke(ctx, profile.BaseURL, profile.Model, apiKey, adapters.InvokeRequest{
			RunID:    runID,
			Messages: body.Messages,
			Options:  adapters.InvokeOptions{Stream: false},
		})
		if err != nil {
			_ = appendTraceFailure(ctx, services.Trace, runID, requestID, "adapter_failed", err.Error())
			writeOpenAIError(response, http.StatusInternalServerError, "adapter_failed", "Provider adapter failed before receiving a response.")
			return
		}
		if result.Status != adapters.StatusCompleted {
			code := "adapter_failed"
			message := "Provider invocation failed."
			status := http.StatusBadGateway
			if result.Error != nil {
				code = result.Error.Code
				message = result.Error.Message
				if result.Error.Code == adapters.ErrorAuthFailed {
					status = http.StatusUnauthorized
				}
			}
			_ = appendTraceFailure(ctx, services.Trace, runID, requestID, code, message)
			writeOpenAIError(response, status, code, message)
			return
		}
		if err := appendV1RunCompleted(ctx, services.Trace, runID, requestID, profile.ID, profile.Model, result); err != nil {
			writeOpenAIError(response, http.StatusInternalServerError, "trace_failed", "Run completion trace could not be written.")
			return
		}

		message := adapters.Message{Role: "assistant", Content: ""}
		if len(result.Messages) > 0 {
			message = result.Messages[0]
		}
		completion := v1ChatCompletionResponse{
			ID:      "chatcmpl_" + runID,
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   body.Model,
			Choices: []v1ChatCompletionChoice{{
				Index:        0,
				Message:      message,
				FinishReason: "stop",
			}},
		}
		if result.Usage != nil {
			completion.Usage = &v1ChatCompletionUsage{
				PromptTokens:     result.Usage.InputTokens,
				CompletionTokens: result.Usage.OutputTokens,
				TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
			}
		}
		writeJSON(response, http.StatusOK, completion)
	}
}

func writeOpenAIError(response http.ResponseWriter, status int, code string, message string) {
	writeJSON(response, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}

func appendV1RunStarted(ctx context.Context, traceStore *trace.Store, runID string, requestID string, providerID string, model string, messages []adapters.Message, redactions []string) error {
	if err := traceStore.Append(ctx, &trace.Event{
		RunID:    runID,
		Type:     trace.EventRunStarted,
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "openai_compatible"}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &trace.Event{
		RunID: runID,
		Type:  trace.EventModelRequested,
		Payload: mustJSON(map[string]any{
			"provider_id": providerID,
			"model":       model,
			"messages":    messages,
		}),
		Redactions: redactions,
		Metadata:   mustJSON(map[string]string{"request_id": requestID, "surface": "openai_compatible"}),
	})
}

func appendV1RunCompleted(ctx context.Context, traceStore *trace.Store, runID string, requestID string, providerID string, model string, result *adapters.InvokeResult) error {
	if err := traceStore.Append(ctx, &trace.Event{
		RunID: runID,
		Type:  trace.EventModelCompleted,
		Payload: mustJSON(map[string]any{
			"provider_id": providerID,
			"model":       model,
			"usage":       result.Usage,
		}),
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "openai_compatible"}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &trace.Event{
		RunID:    runID,
		Type:     trace.EventRunCompleted,
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "openai_compatible"}),
	})
}
