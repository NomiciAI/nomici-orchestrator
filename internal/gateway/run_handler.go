package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/go-chi/chi/v5"
)

type runCreateRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

type runCreateResponse struct {
	RunID           string `json:"run_id"`
	Status          string `json:"status"`
	AgentID         string `json:"agent_id"`
	GraphSnapshotID string `json:"graph_snapshot_id"`
}

type traceEventResponse struct {
	EventID    string          `json:"event_id"`
	RunID      string          `json:"run_id"`
	Sequence   int             `json:"sequence"`
	Type       string          `json:"type"`
	Time       string          `json:"time"`
	NodeID     string          `json:"node_id,omitempty"`
	RuntimeID  string          `json:"runtime_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Redactions []string        `json:"redactions"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

func runCreateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Graph == nil || services.Trace == nil || services.Secrets == nil || services.Adapter == nil || services.Policy == nil || services.Context == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "runs_unavailable", "Run services are not initialized.", "Restart Gateway.")
			return
		}
		var body runCreateRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send agent_id and prompt as JSON.")
			return
		}
		snapshot, err := services.Graph.Latest(request.Context())
		if err != nil {
			writeError(response, http.StatusNotFound, requestID, "graph_not_found", "No compiled graph snapshot was found.", "Run `nomici graph validate` or install a pack.")
			return
		}
		executor := runExecutor(options, services)
		runRequest := runpkg.Request{
			Snapshot: snapshot,
			AgentID:  body.AgentID,
			Prompt:   body.Prompt,
			RunID:    ids.New("run"),
		}
		agent, _, err := executor.Validate(runRequest)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "run_not_supported", err.Error(), "Choose a supported graph entrypoint.")
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if _, err := executor.Execute(ctx, runRequest); err != nil {
				log.Printf("Console run %s failed: %v", runRequest.RunID, err)
			}
		}()

		writeSuccess(response, requestID, runCreateResponse{
			RunID:           runRequest.RunID,
			Status:          "started",
			AgentID:         agent.ID,
			GraphSnapshotID: snapshot.SnapshotID,
		}, nil)
	}
}

func runEventsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "runs_unavailable", "Trace store is not initialized.", "Restart Gateway.")
			return
		}
		runID := chi.URLParam(request, "run_id")
		afterSequence := 0
		if raw := request.URL.Query().Get("after_sequence"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "after_sequence must be a non-negative integer.", "Use the last received event sequence.")
				return
			}
			afterSequence = parsed
		}
		events, err := services.Trace.ListByRunAfter(request.Context(), runID, afterSequence)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_load_failed", "Run trace could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, traceEventsResponse(events), nil)
	}
}

func approvalGrantHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Policy == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "approvals_unavailable", "Approval services are not initialized.", "Restart Gateway.")
			return
		}
		var body struct {
			Scope string `json:"scope"`
		}
		if request.Body != nil {
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send optional scope as JSON.")
				return
			}
		}
		approval, err := services.Policy.Grant(request.Context(), chi.URLParam(request, "approval_id"), body.Scope)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "approval_grant_failed", err.Error(), "Refresh approvals and retry if it is still pending.")
			return
		}
		if err := runpkg.ApprovalTraceEvent(request.Context(), services.Trace, trace.EventApprovalGranted, approval); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Approval trace event could not be written.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, approval, nil)
	}
}

func approvalDenyHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Policy == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "approvals_unavailable", "Approval services are not initialized.", "Restart Gateway.")
			return
		}
		approval, err := services.Policy.Deny(request.Context(), chi.URLParam(request, "approval_id"))
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "approval_deny_failed", err.Error(), "Refresh approvals and retry if it is still pending.")
			return
		}
		if err := runpkg.ApprovalTraceEvent(request.Context(), services.Trace, trace.EventApprovalDenied, approval); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Approval trace event could not be written.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, approval, nil)
	}
}

func runExecutor(options Options, services Services) *runpkg.Executor {
	return &runpkg.Executor{
		Providers:  services.Providers,
		Trace:      services.Trace,
		Secrets:    services.Secrets,
		Adapter:    services.Adapter,
		Policy:     services.Policy,
		Context:    services.Context,
		ConfigPath: options.ConfigPath,
	}
}

func traceEventsResponse(events []*trace.Event) []traceEventResponse {
	response := make([]traceEventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, traceEventResponse{
			EventID:    event.EventID,
			RunID:      event.RunID,
			Sequence:   event.Sequence,
			Type:       event.Type,
			Time:       event.Time.Format(time.RFC3339Nano),
			NodeID:     event.NodeID,
			RuntimeID:  event.RuntimeID,
			Payload:    event.Payload,
			Redactions: event.Redactions,
			Metadata:   event.Metadata,
		})
	}
	return response
}
