package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

type modelTestRequest struct {
	ProviderID string `json:"provider_id"`
	Prompt     string `json:"prompt"`
	Stream     bool   `json:"stream"`
}

type modelTestResponse struct {
	RunID           string              `json:"run_id"`
	Status          string              `json:"status"`
	Messages        []adapters.Message  `json:"messages"`
	Usage           *adapters.UsageInfo `json:"usage,omitempty"`
	TraceEventCount int                 `json:"trace_event_count"`
}

func modelTestHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body modelTestRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send provider_id and prompt as JSON.")
			return
		}
		if body.ProviderID == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "provider_id is required.", "Pass the configured provider profile ID.")
			return
		}
		if body.Prompt == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "prompt is required.", "Pass a non-empty test prompt.")
			return
		}

		ctx := request.Context()
		profile, err := services.Providers.Get(ctx, body.ProviderID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "provider_not_found", "Provider profile was not found.", "Run `nomici model list` to see configured profiles.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "provider_load_failed", "Provider profile could not be loaded.", "Check Gateway logs for details.")
			return
		}

		var apiKey string
		var redactions []string
		if profile.APIKeyEnv != "" {
			resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
			if !ok {
				writeError(response, http.StatusBadRequest, requestID, "missing_secret", "Provider API key environment variable is not set.", "Set "+profile.APIKeyEnv+" and restart or rerun the command.")
				return
			}
			apiKey = resolved
			redactions = append(redactions, secrets.RedactedEnv(profile.APIKeyEnv))
		}

		runID := ids.New("run")
		if err := services.Trace.Append(ctx, &trace.Event{
			RunID:    runID,
			Type:     trace.EventRunStarted,
			Metadata: mustJSON(map[string]string{"request_id": requestID}),
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Run trace could not be created.", "Check Gateway logs for details.")
			return
		}

		if err := services.Trace.Append(ctx, &trace.Event{
			RunID: runID,
			Type:  trace.EventModelRequested,
			Payload: mustJSON(map[string]any{
				"provider_id":    profile.ID,
				"model":          profile.Model,
				"base_url":       profile.BaseURL,
				"prompt":         body.Prompt,
				"api_key_source": profile.APIKeyEnv,
			}),
			Redactions: redactions,
			Metadata:   mustJSON(map[string]string{"request_id": requestID}),
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Model request trace could not be written.", "Check Gateway logs for details.")
			return
		}

		result, err := services.Adapter.Invoke(ctx, adapters.ModelConfig{
			Kind:    profile.Kind,
			BaseURL: profile.BaseURL,
			Model:   profile.Model,
		}, apiKey, adapters.InvokeRequest{
			RunID: runID,
			Messages: []adapters.Message{
				{Role: "user", Content: body.Prompt},
			},
			Options: adapters.InvokeOptions{Stream: body.Stream},
		})
		if err != nil {
			_ = appendTraceFailure(ctx, services.Trace, runID, requestID, "adapter_failed", err.Error())
			writeError(response, http.StatusInternalServerError, requestID, "adapter_failed", "Provider adapter failed before receiving a response.", "Check the provider base URL and Gateway logs.")
			return
		}

		if result.Status != adapters.StatusCompleted {
			code := "adapter_failed"
			message := "Provider invocation failed."
			remediation := "Check provider configuration and Gateway logs."
			status := http.StatusBadGateway
			if result.Error != nil {
				code = result.Error.Code
				message = result.Error.Message
				if result.Error.Code == adapters.ErrorAuthFailed {
					status = http.StatusUnauthorized
					if profile.APIKeyEnv != "" {
						remediation = "Verify the " + profile.APIKeyEnv + " environment variable is set and valid."
					} else {
						remediation = "Run the provider login flow and retry the request."
					}
				}
			}
			_ = appendTraceFailure(ctx, services.Trace, runID, requestID, code, message)
			writeError(response, status, requestID, code, message, remediation)
			return
		}

		if err := services.Trace.Append(ctx, &trace.Event{
			RunID: runID,
			Type:  trace.EventModelCompleted,
			Payload: mustJSON(map[string]any{
				"provider_id": profile.ID,
				"model":       profile.Model,
				"usage":       result.Usage,
			}),
			Metadata: mustJSON(map[string]string{"request_id": requestID}),
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Model completion trace could not be written.", "Check Gateway logs for details.")
			return
		}
		if err := services.Trace.Append(ctx, &trace.Event{
			RunID:    runID,
			Type:     trace.EventRunCompleted,
			Metadata: mustJSON(map[string]string{"request_id": requestID}),
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Run completion trace could not be written.", "Check Gateway logs for details.")
			return
		}

		events, err := services.Trace.ListByRun(ctx, runID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Run trace could not be loaded.", "Check Gateway logs for details.")
			return
		}

		writeSuccess(response, requestID, modelTestResponse{
			RunID:           runID,
			Status:          result.Status,
			Messages:        result.Messages,
			Usage:           result.Usage,
			TraceEventCount: len(events),
		}, nil)
	}
}

func appendTraceFailure(ctx context.Context, traceStore *trace.Store, runID string, requestID string, code string, message string) error {
	if err := traceStore.Append(ctx, &trace.Event{
		RunID: runID,
		Type:  trace.EventModelFailed,
		Payload: mustJSON(map[string]string{
			"code":    code,
			"message": message,
		}),
		Metadata: mustJSON(map[string]string{"request_id": requestID}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &trace.Event{
		RunID:    runID,
		Type:     trace.EventRunFailed,
		Metadata: mustJSON(map[string]string{"request_id": requestID}),
	})
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}
