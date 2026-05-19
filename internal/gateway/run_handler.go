package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
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
	SessionID       string `json:"session_id,omitempty"`
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
		if services.Graph == nil || services.Trace == nil || services.Secrets == nil || services.Adapter == nil || services.Policy == nil || services.Context == nil || services.Runs == nil {
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
		session, taskIDs, err := createRunLedger(request.Context(), services, snapshot, runRequest, agent.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "run_session_failed", "Run session could not be created.", "Check Gateway logs.")
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			result, err := executor.Execute(ctx, runRequest)
			if err != nil {
				_ = completeRunLedger(ctx, services, runRequest.RunID, runpkg.SessionStatusFailed, runpkg.TaskStatusFailed, taskIDs)
				log.Printf("Console run %s failed: %v", runRequest.RunID, err)
				return
			}
			sessionStatus := runpkg.SessionStatusCompleted
			taskStatus := runpkg.TaskStatusCompleted
			if result.Status != "" && result.Status != "completed" {
				sessionStatus = runpkg.SessionStatusFailed
				taskStatus = runpkg.TaskStatusFailed
			}
			_ = completeRunLedger(ctx, services, runRequest.RunID, sessionStatus, taskStatus, taskIDs)
		}()

		writeSuccess(response, requestID, runCreateResponse{
			RunID:           runRequest.RunID,
			Status:          "started",
			AgentID:         agent.ID,
			GraphSnapshotID: snapshot.SnapshotID,
			SessionID:       session.SessionID,
		}, nil)
	}
}

func runDetailHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "runs_unavailable", "Run session store is not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetByRun(request.Context(), chi.URLParam(request, "run_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "run_not_found", "Run session was not found.", "Refresh recent runs.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "run_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, detail, nil)
	}
}

func createRunLedger(ctx context.Context, services Services, snapshot *graph.Snapshot, request runpkg.Request, agentID string) (*runpkg.Session, []string, error) {
	session, err := services.Runs.CreateSession(ctx, runpkg.CreateSessionRequest{
		RunID:           request.RunID,
		ProjectID:       snapshot.ProjectID,
		GraphSnapshotID: snapshot.SnapshotID,
		Title:           request.Prompt,
		SourceChannel:   "console",
		Status:          runpkg.SessionStatusRunning,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventRunSessionCreated, "", map[string]any{
		"session_id":        session.SessionID,
		"project_id":        session.ProjectID,
		"graph_snapshot_id": session.GraphSnapshotID,
		"source_channel":    session.SourceChannel,
		"status":            session.Status,
	}); err != nil {
		return nil, nil, err
	}

	plans := ledgerTaskPlans(snapshot, agentID)
	taskIDs := make([]string, 0, len(plans))
	for index, plan := range plans {
		status := runpkg.TaskStatusQueued
		if index == 0 {
			status = runpkg.TaskStatusRunning
		}
		task, err := services.Runs.CreateTask(ctx, runpkg.CreateTaskRequest{
			RunID:     request.RunID,
			AgentID:   plan.AgentID,
			RuntimeID: plan.RuntimeID,
			Status:    status,
		})
		if err != nil {
			return nil, nil, err
		}
		taskIDs = append(taskIDs, task.TaskID)
		if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCreated, task.AgentID, map[string]any{
			"task_id":    task.TaskID,
			"agent_id":   task.AgentID,
			"runtime_id": task.RuntimeID,
			"status":     task.Status,
		}); err != nil {
			return nil, nil, err
		}
	}
	return session, taskIDs, nil
}

func completeRunLedger(ctx context.Context, services Services, runID string, sessionStatus string, taskStatus string, taskIDs []string) error {
	if services.Runs == nil || services.Trace == nil {
		return nil
	}
	if err := services.Runs.CompleteTasks(ctx, runID, taskStatus); err != nil {
		return err
	}
	eventType := trace.EventTaskCompleted
	if taskStatus == runpkg.TaskStatusFailed {
		eventType = trace.EventTaskFailed
	}
	for _, taskID := range taskIDs {
		if err := appendRunLedgerTrace(ctx, services.Trace, runID, eventType, "", map[string]any{
			"task_id": taskID,
			"status":  taskStatus,
		}); err != nil {
			return err
		}
	}
	if err := services.Runs.CompleteSession(ctx, runID, sessionStatus); err != nil {
		return err
	}
	return appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventRunSessionCompleted, "", map[string]any{
		"status": sessionStatus,
	})
}

type ledgerTaskPlan struct {
	AgentID   string
	RuntimeID string
}

func ledgerTaskPlans(snapshot *graph.Snapshot, startAgentID string) []ledgerTaskPlan {
	plans := []ledgerTaskPlan{}
	visited := map[string]bool{}
	current := startAgentID
	for current != "" && !visited[current] {
		visited[current] = true
		agent, ok := snapshot.IR.Agents[current]
		if !ok {
			break
		}
		plans = append(plans, ledgerTaskPlan{AgentID: agent.ID, RuntimeID: agent.Runtime})
		next := ""
		for _, edge := range snapshot.IR.Edges {
			if edge.From == current && edge.Mode == "handoff" {
				next = edge.To
				break
			}
		}
		current = next
	}
	if len(plans) == 0 {
		plans = append(plans, ledgerTaskPlan{AgentID: startAgentID})
	}
	return plans
}

func appendRunLedgerTrace(ctx context.Context, traceStore *trace.Store, runID string, eventType string, nodeID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return traceStore.Append(ctx, &trace.Event{
		RunID:   runID,
		Type:    eventType,
		NodeID:  nodeID,
		Payload: body,
	})
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
