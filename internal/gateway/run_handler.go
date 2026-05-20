package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	artifactpkg "github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	blockedpkg "github.com/NomiciAI/nomici-orchestrator/internal/blocked"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/memory"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/skills"
	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	uploadpkg "github.com/NomiciAI/nomici-orchestrator/internal/uploads"
	"github.com/go-chi/chi/v5"
)

type runCreateRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

type runCreateResponse struct {
	RunID           string                       `json:"run_id"`
	Status          string                       `json:"status"`
	AgentID         string                       `json:"agent_id"`
	GraphSnapshotID string                       `json:"graph_snapshot_id"`
	SessionID       string                       `json:"session_id,omitempty"`
	SandboxID       string                       `json:"sandbox_id,omitempty"`
	SandboxStatus   string                       `json:"sandbox_status,omitempty"`
	RouteDecision   *orchestration.RouteDecision `json:"route_decision,omitempty"`
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

type runSessionDetailResponse struct {
	Session   *runpkg.Session          `json:"session"`
	Tasks     []*runpkg.Task           `json:"tasks"`
	Sandbox   *sandbox.Record          `json:"sandbox,omitempty"`
	Uploads   []*uploadpkg.Upload      `json:"uploads,omitempty"`
	Artifacts []*artifactpkg.Artifact  `json:"artifacts,omitempty"`
	ToolCalls []*toolbroker.CallRecord `json:"tool_calls,omitempty"`
	Blocked   []*blockedpkg.Action     `json:"blocked_actions,omitempty"`
}

var detectSandboxAvailability = sandbox.Detect

const (
	maxModelToolRounds        = 5
	maxModelToolCallsPerRound = 8
)

type modelToolEnvelope struct {
	ToolCalls []modelToolCall `json:"tool_calls"`
	Final     string          `json:"final,omitempty"`
}

type modelToolCall struct {
	ID     string         `json:"id,omitempty"`
	ToolID string         `json:"tool_id"`
	Input  map[string]any `json:"input,omitempty"`
}

func runCreateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body runCreateRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send agent_id and prompt as JSON.")
			return
		}
		started, startErr := startWorkspaceRun(request.Context(), options, services, body.AgentID, body.Prompt, "console", nil)
		if startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		writeSuccess(response, requestID, started.Response, nil)
	}
}

type startRunResult struct {
	Response runCreateResponse
	Session  *runpkg.Session
	Request  runpkg.Request
	Executor *runpkg.Executor
	Tasks    []*runpkg.Task
}

type startRunError struct {
	Status      int
	Code        string
	Message     string
	Remediation string
}

func startWorkspaceRun(ctx context.Context, options Options, services Services, agentID string, prompt string, sourceChannel string, metadata map[string]any) (*startRunResult, *startRunError) {
	return startWorkspaceRunWithRoute(ctx, options, services, agentID, prompt, sourceChannel, metadata, nil, true)
}

func startWorkspaceRunWithRoute(ctx context.Context, options Options, services Services, agentID string, prompt string, sourceChannel string, metadata map[string]any, routeDecision *orchestration.RouteDecision, autoStart bool) (*startRunResult, *startRunError) {
	if services.Graph == nil || services.Trace == nil || services.Secrets == nil || services.Adapter == nil || services.Policy == nil || services.Context == nil || services.Runs == nil || services.Sandboxes == nil || services.Artifacts == nil {
		return nil, &startRunError{Status: http.StatusServiceUnavailable, Code: "runs_unavailable", Message: "Run services are not initialized.", Remediation: "Restart Gateway."}
	}
	snapshot, err := services.Graph.Latest(ctx)
	if err != nil {
		return nil, &startRunError{Status: http.StatusNotFound, Code: "graph_not_found", Message: "No compiled graph snapshot was found.", Remediation: "Run `nomici graph validate` or install a pack."}
	}
	agentID = strings.TrimSpace(agentID)
	orchestrationConfig, _ := projectconfig.GetOrchestration(options.ConfigPath)
	if agentID == "" && orchestrationConfig.Entrypoint != "" {
		agentID = orchestrationConfig.Entrypoint
	}
	if routeDecision == nil {
		decision := orchestration.Route(prompt, agentID, snapshot)
		routeDecision = &decision
	}
	applyOrchestrationConfig(routeDecision, orchestrationConfig)
	if strings.TrimSpace(routeDecision.RecommendedAgentID) != "" {
		agentID = strings.TrimSpace(routeDecision.RecommendedAgentID)
	}
	if agentID == "" {
		agentID = defaultRunAgent(snapshot)
	}
	executor := runExecutor(options, services)
	runRequest := runpkg.Request{
		Snapshot: snapshot,
		AgentID:  agentID,
		Prompt:   prompt,
		RunID:    ids.New("run"),
	}
	agent, _, err := executor.Validate(runRequest)
	if err != nil {
		return nil, &startRunError{Status: http.StatusBadRequest, Code: "run_not_supported", Message: err.Error(), Remediation: "Choose a supported graph entrypoint."}
	}
	intent, err := sandboxIntentFromConfig(options.ConfigPath)
	if err != nil {
		return nil, &startRunError{Status: http.StatusBadRequest, Code: "sandbox_config_invalid", Message: err.Error(), Remediation: "Run `nomici setup` or fix deployment.sandbox in nomici.yaml."}
	}
	availability := detectSandboxAvailability(intent.Mode)
	if availability.Status == sandbox.StatusUnavailable {
		return nil, &startRunError{Status: http.StatusConflict, Code: "sandbox_unavailable", Message: availability.Message, Remediation: "Install Docker, Podman, or Apple container, or run `nomici setup --sandbox local`."}
	}
	baseDir, err := sandboxBaseDir(options.ConfigPath)
	if err != nil {
		return nil, &startRunError{Status: http.StatusBadRequest, Code: "sandbox_config_invalid", Message: err.Error(), Remediation: "Use a valid AgentSpec config path."}
	}
	session, sandboxRecord, tasks, err := createRunLedger(ctx, services, snapshot, runRequest, agent.ID, intent, baseDir, sourceChannel, metadata, routeDecision)
	if err != nil {
		return nil, &startRunError{Status: http.StatusInternalServerError, Code: "run_session_failed", Message: "Run session could not be created.", Remediation: "Check Gateway logs."}
	}
	if autoStart {
		startRunWorker(options, services, executor, runRequest, session, tasks)
	}
	return &startRunResult{
		Response: runCreateResponse{
			RunID:           runRequest.RunID,
			Status:          "started",
			AgentID:         agent.ID,
			GraphSnapshotID: snapshot.SnapshotID,
			SessionID:       session.SessionID,
			SandboxID:       sandboxRecord.SandboxID,
			SandboxStatus:   sandboxRecord.Status,
			RouteDecision:   routeDecision,
		},
		Session:  session,
		Request:  runRequest,
		Executor: executor,
		Tasks:    tasks,
	}, nil
}

func defaultRunAgent(snapshot *graph.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	if _, ok := snapshot.IR.Agents["product_pm"]; ok {
		return "product_pm"
	}
	var ids []string
	for id := range snapshot.IR.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func applyOrchestrationConfig(routeDecision *orchestration.RouteDecision, config projectconfig.OrchestrationConfig) {
	if routeDecision == nil {
		return
	}
	if config.Entrypoint != "" && routeDecision.ManualAgentID == "" {
		routeDecision.RecommendedAgentID = config.Entrypoint
	}
	if len(config.RoleOrder) > 0 {
		disabled := map[string]bool{}
		for _, role := range config.DisabledRoles {
			disabled[role] = true
		}
		roles := []string{}
		for _, role := range config.RoleOrder {
			if role != "" && !disabled[role] {
				roles = append(roles, role)
			}
		}
		routeDecision.SelectedRoles = roles
	}
	switch strings.ToLower(config.PlanReviewPolicy) {
	case "always", "required":
		routeDecision.NeedsPlanReview = true
	case "never", "disabled":
		routeDecision.NeedsPlanReview = false
	}
}

func startRunWorker(options Options, services Services, executor *runpkg.Executor, request runpkg.Request, session *runpkg.Session, tasks []*runpkg.Task) {
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := executeWorkspaceSession(runCtx, options, services, executor, request, session, tasks); err != nil {
			if current, lookupErr := services.Runs.GetBySession(runCtx, session.SessionID); lookupErr == nil {
				switch current.Session.Status {
				case runpkg.SessionStatusCancelled:
					log.Printf("Console run %s cancelled", request.RunID)
					return
				case runpkg.SessionStatusPlanReview, runpkg.SessionStatusBlocked, runpkg.SessionStatusNeedsClarification:
					log.Printf("Console run %s paused: %s", request.RunID, current.Session.Status)
					return
				}
			}
			_ = completeRunLedger(runCtx, services, request.RunID, runpkg.SessionStatusFailed, runpkg.TaskStatusFailed, taskIDs(tasks))
			log.Printf("Console run %s failed: %v", request.RunID, err)
			return
		}
	}()
}

func resumeWorkspaceWorker(ctx context.Context, options Options, services Services, detail *runpkg.SessionDetail) *startRunError {
	if services.Graph == nil || services.Runs == nil || services.Trace == nil {
		return &startRunError{Status: http.StatusServiceUnavailable, Code: "runs_unavailable", Message: "Run services are not initialized.", Remediation: "Restart Gateway."}
	}
	snapshot, err := services.Graph.Get(ctx, detail.Session.GraphSnapshotID)
	if err != nil {
		return &startRunError{Status: http.StatusNotFound, Code: "graph_not_found", Message: "Compiled graph snapshot for this session was not found.", Remediation: "Recompile the graph and start a new session."}
	}
	request := runpkg.Request{
		Snapshot: snapshot,
		AgentID:  firstTaskAgent(detail.Tasks),
		Prompt:   detail.Session.Title,
		RunID:    detail.Session.RunID,
	}
	startRunWorker(options, services, runExecutor(options, services), request, detail.Session, detail.Tasks)
	return nil
}

func firstTaskAgent(tasks []*runpkg.Task) string {
	for _, task := range tasks {
		if task.AgentID != "" {
			return task.AgentID
		}
	}
	return ""
}

func runDetailHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Sandboxes == nil {
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
		payload, err := sessionDetailPayload(request.Context(), services, detail)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "sandbox_load_failed", "Sandbox record could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session store is not initialized.", "Restart Gateway.")
			return
		}
		limit := consoleRunLimit
		if raw := request.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "limit must be between 1 and 100.", "Use a positive integer limit.")
				return
			}
			limit = parsed
		}
		sessions, err := services.Runs.ListSessions(request.Context(), limit)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "sessions_list_failed", "Run sessions could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, sessions, nil)
	}
}

func sessionDetailHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Sandboxes == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session store is not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		payload, err := sessionDetailPayload(request.Context(), services, detail)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "sandbox_load_failed", "Sandbox record could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionTasksHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session store is not initialized.", "Restart Gateway.")
			return
		}
		tasks, err := services.Runs.ListTasksBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "session_tasks_failed", "Run session tasks could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, tasks, nil)
	}
}

func sessionCancelHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session services are not initialized.", "Restart Gateway.")
			return
		}
		sessionID := chi.URLParam(request, "session_id")
		detail, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		if err := services.Runs.CancelSession(request.Context(), sessionID); err != nil {
			writeError(response, http.StatusConflict, requestID, "session_not_cancellable", err.Error(), "Only running sessions can be cancelled.")
			return
		}
		if err := services.Runs.CancelTasks(request.Context(), detail.Session.RunID); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_cancel_failed", "Run session tasks could not be cancelled.", "Check Gateway logs.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventRunSessionCompleted, "", map[string]any{
			"session_id": sessionID,
			"status":     runpkg.SessionStatusCancelled,
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_cancel_trace_failed", "Run session cancellation could not be traced.", "Check Gateway logs.")
			return
		}
		updated, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		payload, err := sessionDetailPayload(request.Context(), services, updated)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "sandbox_load_failed", "Sandbox record could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionEventsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session services are not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		afterSequence := 0
		if raw := request.URL.Query().Get("after_sequence"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "after_sequence must be a non-negative integer.", "Use the last received event sequence.")
				return
			}
			afterSequence = parsed
		}
		response.Header().Set("Content-Type", "text/event-stream")
		response.Header().Set("Cache-Control", "no-cache")
		response.Header().Set("Connection", "keep-alive")
		flusher, _ := response.(http.Flusher)
		deadline := time.NewTimer(30 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			events, err := services.Trace.ListByRunAfter(request.Context(), detail.Session.RunID, afterSequence)
			if err != nil {
				payload, _ := json.Marshal(map[string]any{"message": "events could not be loaded"})
				fmt.Fprintf(response, "event: error\ndata: %s\n\n", payload)
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			for _, event := range traceEventsResponse(events) {
				payload, err := json.Marshal(event)
				if err != nil {
					continue
				}
				fmt.Fprintf(response, "event: trace\ndata: %s\n\n", payload)
				if event.Sequence > afterSequence {
					afterSequence = event.Sequence
				}
			}
			if len(events) > 0 && flusher != nil {
				flusher.Flush()
			}
			select {
			case <-request.Context().Done():
				return
			case <-deadline.C:
				fmt.Fprint(response, "event: heartbeat\ndata: {}\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				return
			case <-ticker.C:
			}
		}
	}
}

func sessionDetailPayload(ctx context.Context, services Services, detail *runpkg.SessionDetail) (*runSessionDetailResponse, error) {
	var sandboxRecord *sandbox.Record
	if services.Sandboxes != nil {
		record, err := services.Sandboxes.GetByRun(ctx, detail.Session.RunID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		sandboxRecord = record
	}
	var uploads []*uploadpkg.Upload
	if services.Uploads != nil {
		records, err := services.Uploads.List(ctx, detail.Session.SessionID, 50)
		if err != nil {
			return nil, err
		}
		uploads = records
	}
	var artifacts []*artifactpkg.Artifact
	if services.Artifacts != nil {
		records, err := services.Artifacts.List(ctx, detail.Session.SessionID, 50)
		if err != nil {
			return nil, err
		}
		artifacts = records
	}
	var toolCalls []*toolbroker.CallRecord
	if services.Tools != nil {
		records, err := services.Tools.ListBySession(ctx, detail.Session.SessionID, 50)
		if err != nil {
			return nil, err
		}
		toolCalls = records
	}
	var blockedActions []*blockedpkg.Action
	if services.Blocked != nil {
		records, err := services.Blocked.ListBySession(ctx, detail.Session.SessionID, "", 50)
		if err != nil {
			return nil, err
		}
		blockedActions = records
	}
	return &runSessionDetailResponse{Session: detail.Session, Tasks: detail.Tasks, Sandbox: sandboxRecord, Uploads: uploads, Artifacts: artifacts, ToolCalls: toolCalls, Blocked: blockedActions}, nil
}

func createRunLedger(ctx context.Context, services Services, snapshot *graph.Snapshot, request runpkg.Request, agentID string, intent sandbox.Intent, baseDir string, sourceChannel string, metadata map[string]any, routeDecision *orchestration.RouteDecision) (*runpkg.Session, *sandbox.Record, []*runpkg.Task, error) {
	if sourceChannel == "" {
		sourceChannel = "console"
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	plans, err := ledgerTaskPlans(ctx, services, snapshot, agentID, routeDecision)
	if err != nil {
		return nil, nil, nil, err
	}
	if routeDecision != nil {
		metadata["route_decision"] = routeDecision
		metadata["recommended_agent_id"] = routeDecision.RecommendedAgentID
		metadata["needs_plan_review"] = routeDecision.NeedsPlanReview
	}
	metadataPayload := json.RawMessage("{}")
	if len(metadata) > 0 {
		payload, err := json.Marshal(metadata)
		if err != nil {
			return nil, nil, nil, err
		}
		metadataPayload = payload
	}
	session, err := services.Runs.CreateSession(ctx, runpkg.CreateSessionRequest{
		RunID:           request.RunID,
		ProjectID:       snapshot.ProjectID,
		GraphSnapshotID: snapshot.SnapshotID,
		Title:           request.Prompt,
		SourceChannel:   sourceChannel,
		Status:          runpkg.SessionStatusRunning,
		Metadata:        metadataPayload,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventRunSessionCreated, "", map[string]any{
		"session_id":        session.SessionID,
		"project_id":        session.ProjectID,
		"graph_snapshot_id": session.GraphSnapshotID,
		"source_channel":    session.SourceChannel,
		"status":            session.Status,
	}); err != nil {
		return nil, nil, nil, err
	}

	tasks := make([]*runpkg.Task, 0, len(plans))
	parentTaskID := ""
	for _, plan := range plans {
		status := runpkg.TaskStatusQueued
		metadata, err := json.Marshal(plan.Metadata)
		if err != nil {
			return nil, nil, nil, err
		}
		task, err := services.Runs.CreateTask(ctx, runpkg.CreateTaskRequest{
			RunID:        request.RunID,
			ParentTaskID: parentTaskID,
			AgentID:      plan.AgentID,
			RuntimeID:    plan.RuntimeID,
			Status:       status,
			Metadata:     metadata,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		tasks = append(tasks, task)
		parentTaskID = task.TaskID
		if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCreated, task.AgentID, map[string]any{
			"task_id":        task.TaskID,
			"parent_task_id": task.ParentTaskID,
			"agent_id":       task.AgentID,
			"runtime_id":     task.RuntimeID,
			"status":         task.Status,
			"metadata":       plan.Metadata,
		}); err != nil {
			return nil, nil, nil, err
		}
	}
	taskID := ""
	if len(tasks) > 0 {
		taskID = tasks[0].TaskID
	}
	sandboxProvider := sandboxProviderForIntent(services.Sandboxes, intent)
	sandboxRecord, err := sandboxProvider.Acquire(ctx, sandbox.CreateRecordRequest{
		RunID:     request.RunID,
		TaskID:    taskID,
		ProjectID: snapshot.ProjectID,
		Intent:    intent,
		BaseDir:   baseDir,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventSandboxCreated, "", map[string]any{
		"sandbox_id":     sandboxRecord.SandboxID,
		"provider":       sandboxRecord.Provider,
		"mode":           sandboxRecord.Mode,
		"status":         sandboxRecord.Status,
		"workspace_root": sandboxRecord.WorkspaceRoot,
		"artifact_root":  sandboxRecord.ArtifactRoot,
		"runtime_binary": sandboxRecord.RuntimeBinary,
	}); err != nil {
		return nil, nil, nil, err
	}
	return session, sandboxRecord, tasks, nil
}

func sandboxProviderForIntent(store *sandbox.Store, intent sandbox.Intent) sandbox.Provider {
	if intent.Mode == sandbox.ModeContainer {
		return sandbox.NewContainerProvider(store)
	}
	return sandbox.NewLocalProvider(store)
}

func executeWorkspaceSession(ctx context.Context, options Options, services Services, executor *runpkg.Executor, request runpkg.Request, session *runpkg.Session, tasks []*runpkg.Task) error {
	var summaries []string
	for index, task := range tasks {
		if task.Status == runpkg.TaskStatusCompleted {
			if summary := taskMetadataString(task, "summary"); summary != "" {
				summaries = append(summaries, fmt.Sprintf("%s: %s", task.AgentID, summary))
			}
			continue
		}
		if task.Status == runpkg.TaskStatusCancelled || task.Status == runpkg.TaskStatusFailed {
			return fmt.Errorf("task %s is %s", task.TaskID, task.Status)
		}
		if task.Status == runpkg.TaskStatusBlocked && taskMetadataString(task, "tool_loop_prompt") != "" {
			if err := stopIfSessionRunnable(ctx, services, session.SessionID); err != nil {
				return err
			}
			if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusRunning); err != nil {
				return err
			}
			roleRequest := request
			roleRequest.AgentID = task.AgentID
			prompt := taskMetadataString(task, "tool_loop_prompt")
			if pending := taskMetadataString(task, "tool_loop_pending_json"); pending != "" {
				resumeObservations, err := executePendingModelToolCalls(ctx, options, services, session, task, pending)
				if err != nil {
					return err
				}
				prompt = appendToolObservations(prompt, resumeObservations)
				if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
					"tool_loop_prompt":       prompt,
					"tool_loop_pending_json": "",
				}); err != nil {
					return err
				}
			}
			result, toolSummaries, err := executeTaskWithToolLoop(ctx, options, services, executor, roleRequest, session, task, prompt)
			if err != nil {
				return err
			}
			preview := resultPreview(result)
			summary := taskSummary(task, preview)
			if len(toolSummaries) > 0 {
				summary = strings.Join(append([]string{summary}, toolSummaries...), "\n")
			}
			if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
				"summary":                summary,
				"output_preview":         preview,
				"tool_loop_prompt":       "",
				"tool_loop_pending_json": "",
			}); err != nil {
				return err
			}
			summaries = append(summaries, fmt.Sprintf("%s: %s", task.AgentID, summary))
			if err := createRoleArtifact(ctx, services, session, task, preview, index == len(tasks)-1); err != nil {
				return err
			}
			if taskRoleID(task) == "planner" && routeNeedsPlanReview(session) {
				return blockForPlanReview(ctx, services, session, task, request.RunID)
			}
			if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusCompleted); err != nil {
				return err
			}
			if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCompleted, task.AgentID, map[string]any{
				"task_id": task.TaskID,
				"status":  runpkg.TaskStatusCompleted,
				"summary": summary,
				"reason":  "tool_loop_resumed",
			}); err != nil {
				return err
			}
			var nextTask *runpkg.Task
			if index+1 < len(tasks) {
				nextTask = tasks[index+1]
			}
			if err := saveRoleContextSnapshot(ctx, services, session, task, nextTask, summary); err != nil {
				return err
			}
			continue
		}
		if task.Status == runpkg.TaskStatusPlanReview {
			if err := stopIfSessionRunnable(ctx, services, session.SessionID); err != nil {
				return err
			}
			summary := taskMetadataString(task, "summary")
			if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusCompleted); err != nil {
				return err
			}
			if summary != "" {
				summaries = append(summaries, fmt.Sprintf("%s: %s", task.AgentID, summary))
			}
			if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCompleted, task.AgentID, map[string]any{
				"task_id": task.TaskID,
				"status":  runpkg.TaskStatusCompleted,
				"summary": summary,
				"reason":  "plan_approved",
			}); err != nil {
				return err
			}
			var nextTask *runpkg.Task
			if index+1 < len(tasks) {
				nextTask = tasks[index+1]
			}
			if err := saveRoleContextSnapshot(ctx, services, session, task, nextTask, summary); err != nil {
				return err
			}
			continue
		}
		if task.Status == runpkg.TaskStatusBlocked && taskMetadataString(task, "output_preview") != "" {
			if err := stopIfSessionRunnable(ctx, services, session.SessionID); err != nil {
				return err
			}
			postToolSummaries, err := executeRoleToolPreparation(ctx, options, services, session, task, "post")
			if err != nil {
				return err
			}
			summary := taskMetadataString(task, "summary")
			if len(postToolSummaries) > 0 {
				summary = strings.Join(append([]string{summary}, postToolSummaries...), "\n")
				if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
					"summary": summary,
				}); err != nil {
					return err
				}
			}
			if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusCompleted); err != nil {
				return err
			}
			if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCompleted, task.AgentID, map[string]any{
				"task_id": task.TaskID,
				"status":  runpkg.TaskStatusCompleted,
				"summary": summary,
				"reason":  "tool_approval_resolved",
			}); err != nil {
				return err
			}
			var nextTask *runpkg.Task
			if index+1 < len(tasks) {
				nextTask = tasks[index+1]
			}
			if err := saveRoleContextSnapshot(ctx, services, session, task, nextTask, summary); err != nil {
				return err
			}
			if summary != "" {
				summaries = append(summaries, fmt.Sprintf("%s: %s", task.AgentID, summary))
			}
			continue
		}
		if err := stopIfSessionRunnable(ctx, services, session.SessionID); err != nil {
			return err
		}
		if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusRunning); err != nil {
			return err
		}
		if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskStarted, task.AgentID, map[string]any{
			"task_id": task.TaskID,
			"status":  runpkg.TaskStatusRunning,
		}); err != nil {
			return err
		}
		preToolSummaries, err := executeRoleToolPreparation(ctx, options, services, session, task, "pre")
		if err != nil {
			return err
		}
		roleRequest := request
		roleRequest.AgentID = task.AgentID
		roleRequest.Prompt = rolePrompt(ctx, options.ConfigPath, services, session, request.Prompt, task, append(summaries, preToolSummaries...))
		result, toolSummaries, err := executeTaskWithToolLoop(ctx, options, services, executor, roleRequest, session, task, roleRequest.Prompt)
		if err != nil {
			if sessionIsPaused(ctx, services, session.SessionID) {
				return err
			}
			_ = updateTaskMetadata(ctx, services.Runs, task, map[string]any{
				"summary":        err.Error(),
				"failure_reason": err.Error(),
			})
			_ = services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusFailed)
			_ = appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskFailed, task.AgentID, map[string]any{
				"task_id": task.TaskID,
				"status":  runpkg.TaskStatusFailed,
				"error":   err.Error(),
			})
			return err
		}
		preview := resultPreview(result)
		summary := taskSummary(task, preview)
		if len(toolSummaries) > 0 {
			summary = strings.Join(append([]string{summary}, toolSummaries...), "\n")
		}
		if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
			"summary":        summary,
			"output_preview": preview,
		}); err != nil {
			return err
		}
		summaries = append(summaries, fmt.Sprintf("%s: %s", task.AgentID, summary))
		if err := createRoleArtifact(ctx, services, session, task, preview, index == len(tasks)-1); err != nil {
			return err
		}
		postToolSummaries, err := executeRoleToolPreparation(ctx, options, services, session, task, "post")
		if err != nil {
			return err
		}
		if len(postToolSummaries) > 0 {
			summary = strings.Join(append([]string{summary}, postToolSummaries...), "\n")
			if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
				"summary": summary,
			}); err != nil {
				return err
			}
		}
		if taskRoleID(task) == "planner" && routeNeedsPlanReview(session) {
			return blockForPlanReview(ctx, services, session, task, request.RunID)
		}
		if err := services.Runs.UpdateTaskStatus(ctx, task.TaskID, runpkg.TaskStatusCompleted); err != nil {
			return err
		}
		if err := appendRunLedgerTrace(ctx, services.Trace, request.RunID, trace.EventTaskCompleted, task.AgentID, map[string]any{
			"task_id": task.TaskID,
			"status":  runpkg.TaskStatusCompleted,
			"summary": summary,
		}); err != nil {
			return err
		}
		var nextTask *runpkg.Task
		if index+1 < len(tasks) {
			nextTask = tasks[index+1]
		}
		if err := saveRoleContextSnapshot(ctx, services, session, task, nextTask, summary); err != nil {
			return err
		}
	}
	return completeSessionLifecycle(ctx, services, request.RunID, runpkg.SessionStatusCompleted)
}

func sessionIsPaused(ctx context.Context, services Services, sessionID string) bool {
	if services.Runs == nil || sessionID == "" {
		return false
	}
	detail, err := services.Runs.GetBySession(ctx, sessionID)
	if err != nil {
		return false
	}
	switch detail.Session.Status {
	case runpkg.SessionStatusPlanReview, runpkg.SessionStatusBlocked, runpkg.SessionStatusNeedsClarification:
		return true
	default:
		return false
	}
}

func saveRoleContextSnapshot(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, nextTask *runpkg.Task, summary string) error {
	if services.Context == nil || summary == "" {
		return nil
	}
	toAgent := ""
	if nextTask != nil {
		toAgent = nextTask.AgentID
	}
	snapshot := &sharedcontext.Snapshot{
		ProjectID: session.ProjectID,
		RunID:     session.RunID,
		TaskID:    task.TaskID,
		FromAgent: task.AgentID,
		ToAgent:   toAgent,
		Summary:   summary,
		CreatedBy: sharedcontext.CreatedBy{Kind: "gateway_generated", AgentID: task.AgentID},
	}
	if err := services.Context.SaveSnapshot(ctx, snapshot); err != nil {
		return err
	}
	if err := services.Runs.SetTaskContextSnapshot(ctx, task.TaskID, snapshot.SnapshotID); err != nil {
		return err
	}
	if nextTask != nil {
		if err := services.Runs.SetTaskSelectedContextSnapshot(ctx, nextTask.TaskID, snapshot.SnapshotID); err != nil {
			return err
		}
	}
	return appendRunLedgerTrace(ctx, services.Trace, session.RunID, trace.EventContextSnapshotCreated, task.AgentID, map[string]any{
		"snapshot_id": snapshot.SnapshotID,
		"task_id":     task.TaskID,
		"to_agent":    toAgent,
	})
}

func completeSessionLifecycle(ctx context.Context, services Services, runID string, sessionStatus string) error {
	if err := services.Runs.CompleteSession(ctx, runID, sessionStatus); err != nil {
		return err
	}
	if services.Sandboxes != nil {
		if err := services.Sandboxes.ReleaseByRun(ctx, runID); err != nil {
			return err
		}
		if err := appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventSandboxReleased, "", map[string]any{
			"status": "released",
		}); err != nil {
			return err
		}
	}
	if sessionStatus == runpkg.SessionStatusCompleted {
		if err := createSessionMemoryProposal(ctx, services, runID); err != nil {
			return err
		}
	}
	return appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventRunSessionCompleted, "", map[string]any{
		"status": sessionStatus,
	})
}

func createSessionMemoryProposal(ctx context.Context, services Services, runID string) error {
	if services.Memory == nil || services.Runs == nil {
		return nil
	}
	detail, err := services.Runs.GetByRun(ctx, runID)
	if err != nil {
		return err
	}
	body := ""
	artifactRefs := []string{}
	if services.Artifacts != nil {
		records, err := services.Artifacts.List(ctx, detail.Session.SessionID, 20)
		if err != nil {
			return err
		}
		for _, artifact := range records {
			artifactRefs = append(artifactRefs, artifact.ArtifactID)
			if artifact.Type == artifactpkg.TypeReport && body == "" {
				body = artifact.Preview
			}
		}
	}
	if body == "" {
		for _, task := range detail.Tasks {
			if summary := taskMetadataString(task, "summary"); summary != "" {
				body += task.AgentID + ": " + summary + "\n"
			}
		}
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	proposal, err := services.Memory.Create(ctx, memory.CreateRequest{
		ProjectID:    detail.Session.ProjectID,
		SessionID:    detail.Session.SessionID,
		RunID:        detail.Session.RunID,
		SourceType:   "session_summary",
		Title:        "Reusable context from " + trimMemoryTitle(detail.Session.Title),
		Body:         body,
		ArtifactRefs: artifactRefs,
	})
	if err != nil {
		return err
	}
	return appendRunLedgerTrace(ctx, services.Trace, runID, "memory.proposal.created", "", map[string]any{
		"proposal_id": proposal.ProposalID,
		"status":      proposal.Status,
		"title":       proposal.Title,
	})
}

func trimMemoryTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 64 {
		return value
	}
	return value[:61] + "..."
}

func blockForPlanReview(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, runID string) error {
	if err := services.Runs.UpdateSessionStatus(ctx, runID, runpkg.SessionStatusPlanReview); err != nil {
		return err
	}
	if err := services.Runs.SetTaskBlocked(ctx, task.TaskID, runpkg.TaskStatusPlanReview, "plan_review"); err != nil {
		return err
	}
	artifactID := ""
	if services.Artifacts != nil {
		if latest, err := latestPlanArtifactID(ctx, services, session.SessionID); err == nil {
			artifactID = latest
		}
	}
	if services.Blocked != nil {
		_, _ = services.Blocked.Create(ctx, blockedpkg.CreateRequest{
			SessionID:          session.SessionID,
			RunID:              runID,
			TaskID:             task.TaskID,
			Kind:               blockedpkg.KindPlanReview,
			Title:              "Review plan",
			Body:               "Approve or revise the plan before execution continues.",
			RequiredAction:     "approve_plan",
			ResumeTargetTaskID: task.TaskID,
			ArtifactID:         artifactID,
		})
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventTaskBlocked, "", map[string]any{
		"session_id":  session.SessionID,
		"task_id":     task.TaskID,
		"status":      runpkg.SessionStatusPlanReview,
		"reason":      "plan_review",
		"artifact_id": artifactID,
	}); err != nil {
		return err
	}
	return nil
}

func blockForToolApproval(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, record *toolbroker.CallRecord) error {
	if record == nil {
		return fmt.Errorf("tool approval is pending")
	}
	reason := "tool_approval"
	if record.ApprovalID != "" {
		reason += ":" + record.ApprovalID
	}
	if err := services.Runs.UpdateSessionStatus(ctx, session.RunID, runpkg.SessionStatusBlocked); err != nil {
		return err
	}
	if err := services.Runs.SetTaskBlocked(ctx, task.TaskID, runpkg.TaskStatusBlocked, reason); err != nil {
		return err
	}
	if services.Blocked != nil {
		_, _ = services.Blocked.Create(ctx, blockedpkg.CreateRequest{
			SessionID:          session.SessionID,
			RunID:              session.RunID,
			TaskID:             task.TaskID,
			Kind:               blockedpkg.KindToolApproval,
			Title:              "Approve tool call",
			Body:               "Grant or deny the requested tool call, then resume the session.",
			RequiredAction:     "grant_or_deny_approval",
			ResumeTargetTaskID: task.TaskID,
			ApprovalID:         record.ApprovalID,
			ToolCallID:         record.ToolCallID,
			Metadata: rawJSON(map[string]any{
				"tool_id": record.ToolID,
				"risk":    record.Risk,
			}),
		})
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, session.RunID, trace.EventTaskBlocked, task.AgentID, map[string]any{
		"session_id":    session.SessionID,
		"task_id":       task.TaskID,
		"status":        runpkg.SessionStatusBlocked,
		"reason":        "tool_approval",
		"tool_call_id":  record.ToolCallID,
		"tool_id":       record.ToolID,
		"approval_id":   record.ApprovalID,
		"approval_hint": "Grant or deny the approval, then resume the session.",
	}); err != nil {
		return err
	}
	return fmt.Errorf("tool approval pending: %s", record.ApprovalID)
}

func blockForToolLoopGuardrail(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, reason string, body string, metadata map[string]any) error {
	if session == nil || task == nil {
		return errors.New(reason)
	}
	if err := services.Runs.UpdateSessionStatus(ctx, session.RunID, runpkg.SessionStatusBlocked); err != nil {
		return err
	}
	if err := services.Runs.SetTaskBlocked(ctx, task.TaskID, runpkg.TaskStatusBlocked, reason); err != nil {
		return err
	}
	if services.Blocked != nil {
		_, _ = services.Blocked.Create(ctx, blockedpkg.CreateRequest{
			SessionID:          session.SessionID,
			RunID:              session.RunID,
			TaskID:             task.TaskID,
			Kind:               blockedpkg.KindRetryDecision,
			Title:              "Review tool loop",
			Body:               body,
			RequiredAction:     "retry_skip_or_stop",
			ResumeTargetTaskID: task.TaskID,
			Metadata:           rawJSON(metadata),
		})
	}
	if err := updateTaskMetadata(ctx, services.Runs, task, map[string]any{
		"summary":        body,
		"failure_reason": reason,
	}); err != nil {
		return err
	}
	if err := appendRunLedgerTrace(ctx, services.Trace, session.RunID, trace.EventTaskBlocked, task.AgentID, map[string]any{
		"session_id": session.SessionID,
		"task_id":    task.TaskID,
		"status":     runpkg.SessionStatusBlocked,
		"reason":     reason,
		"metadata":   metadata,
	}); err != nil {
		return err
	}
	return fmt.Errorf("%s: %s", reason, body)
}

func executeRoleToolPreparation(ctx context.Context, options Options, services Services, session *runpkg.Session, task *runpkg.Task, phase string) ([]string, error) {
	if services.Tools == nil || session == nil || task == nil {
		return nil, nil
	}
	required := taskMetadataStringSlice(task, "required_tools")
	if len(required) == 0 {
		return nil, nil
	}
	broker := newToolBroker(options, services)
	summaries := []string{}
	for _, tool := range required {
		switch {
		case phase == "pre" && tool == "read_project":
			record, err := broker.Execute(ctx, toolbroker.ExecuteRequest{
				SessionID: session.SessionID,
				RunID:     session.RunID,
				TaskID:    task.TaskID,
				AgentID:   task.AgentID,
				ToolID:    toolbroker.ToolListFiles,
				Input:     map[string]any{"path": ".", "limit": 80},
			})
			if err != nil {
				summaries = append(summaries, "Tool list_files failed: "+err.Error())
				continue
			}
			if record != nil && record.OutputPreview != "" {
				summaries = append(summaries, "Workspace files:\n"+record.OutputPreview)
			}
		case phase == "post" && tool == "run_checks":
			record, err := broker.Execute(ctx, toolbroker.ExecuteRequest{
				SessionID: session.SessionID,
				RunID:     session.RunID,
				TaskID:    task.TaskID,
				AgentID:   task.AgentID,
				ToolID:    toolbroker.ToolBash,
				Input:     map[string]any{"command": "make test", "cwd": ".", "timeout_seconds": 300},
			})
			if record != nil && record.Status == toolbroker.StatusWaitingApproval {
				return summaries, blockForToolApproval(ctx, services, session, task, record)
			}
			if err != nil {
				summaries = append(summaries, "Verification tool failed:\n"+err.Error())
				if record != nil && record.OutputPreview != "" {
					summaries = append(summaries, record.OutputPreview)
				}
				continue
			}
			if record != nil && record.OutputPreview != "" {
				summaries = append(summaries, "Verification tool output:\n"+record.OutputPreview)
			}
		}
	}
	return summaries, nil
}

func executeTaskWithToolLoop(ctx context.Context, options Options, services Services, executor *runpkg.Executor, request runpkg.Request, session *runpkg.Session, task *runpkg.Task, prompt string) (*runpkg.Result, []string, error) {
	if services.Tools == nil || session == nil || task == nil {
		result, err := executor.ExecuteTask(ctx, request, task.TaskID)
		return result, nil, err
	}
	broker := newToolBroker(options, services)
	observations := []string{}
	currentPrompt := prompt
	failureCounts := map[string]int{}
	for round := 0; round <= maxModelToolRounds; round++ {
		if err := stopIfSessionRunnable(ctx, services, session.SessionID); err != nil {
			return nil, observations, err
		}
		roundRequest := request
		roundRequest.Prompt = currentPrompt
		roundRequest.Tools = modelToolSchemas()
		result, err := executor.ExecuteTask(ctx, roundRequest, task.TaskID)
		if err != nil {
			return result, observations, err
		}
		preview := resultPreview(result)
		calls, ok := modelToolCallsFromResult(result)
		if !ok || len(calls) == 0 {
			_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, "tool.loop.completed", task.AgentID, map[string]any{
				"task_id":       task.TaskID,
				"rounds":        round,
				"observations":  len(observations),
				"final_preview": sharedcontext.RedactText(preview),
			})
			return result, observations, nil
		}
		if round == maxModelToolRounds {
			return result, observations, blockForToolLoopGuardrail(ctx, services, session, task, "tool_loop_budget_exhausted", fmt.Sprintf("Tool loop reached the %d round budget. Review the observations and choose whether to retry, revise the plan, or stop.", maxModelToolRounds), map[string]any{
				"rounds":       maxModelToolRounds,
				"observations": redactedStrings(observations),
			})
		}
		if len(calls) > maxModelToolCallsPerRound {
			calls = calls[:maxModelToolCallsPerRound]
		}
		_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, "tool.loop.requested", task.AgentID, map[string]any{
			"task_id": task.TaskID,
			"round":   round + 1,
			"count":   len(calls),
		})
		roundObservations := []string{}
		for _, call := range calls {
			toolID := normalizeModelToolID(call.ToolID)
			if toolID == "" {
				roundObservations = append(roundObservations, fmt.Sprintf("Tool %q was skipped because it is not available.", call.ToolID))
				continue
			}
			record, execErr := broker.Execute(ctx, toolbroker.ExecuteRequest{
				SessionID: session.SessionID,
				RunID:     session.RunID,
				TaskID:    task.TaskID,
				AgentID:   task.AgentID,
				ToolID:    toolID,
				Input:     call.Input,
			})
			if record != nil && record.Status == toolbroker.StatusWaitingApproval {
				pendingPrompt := appendToolObservations(currentPrompt, roundObservations)
				pendingJSON, _ := json.Marshal([]modelToolCall{call})
				_ = updateTaskMetadata(ctx, services.Runs, task, map[string]any{
					"summary":                "Waiting for approval to run " + toolID + ".",
					"output_preview":         preview,
					"tool_loop_prompt":       pendingPrompt,
					"tool_loop_pending_json": string(pendingJSON),
					"tool_loop_round":        round + 1,
				})
				_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, "tool.loop.blocked", task.AgentID, map[string]any{
					"task_id":      task.TaskID,
					"round":        round + 1,
					"tool_id":      toolID,
					"tool_call_id": record.ToolCallID,
					"approval_id":  record.ApprovalID,
				})
				return result, observations, blockForToolApproval(ctx, services, session, task, record)
			}
			observation := toolObservation(toolID, record, execErr)
			roundObservations = append(roundObservations, observation)
			_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, "tool.loop.observed", task.AgentID, map[string]any{
				"task_id":        task.TaskID,
				"tool_id":        toolID,
				"tool_call_id":   toolCallID(record),
				"status":         toolCallStatus(record, execErr),
				"output_preview": sharedcontext.RedactText(observation),
			})
			if execErr != nil {
				if record != nil && record.Risk == toolbroker.RiskCritical {
					return result, append(observations, roundObservations...), blockForToolRiskReview(ctx, services, session, task, record, execErr)
				}
				signature := modelToolFailureSignature(toolID, call.Input)
				failureCounts[signature]++
				if failureCounts[signature] >= 2 {
					return result, append(observations, roundObservations...), blockForToolLoopGuardrail(ctx, services, session, task, "repeated_tool_failure", "The same tool request failed repeatedly. Choose whether to retry after changing inputs, skip the tool, or stop the run.", map[string]any{
						"tool_id":       toolID,
						"tool_call_id":  toolCallID(record),
						"failure_count": failureCounts[signature],
						"error":         sharedcontext.RedactText(execErr.Error()),
						"observation":   sharedcontext.RedactText(observation),
					})
				}
			}
		}
		observations = append(observations, roundObservations...)
		currentPrompt = appendToolObservations(currentPrompt, roundObservations)
	}
	return nil, observations, fmt.Errorf("tool loop did not produce a final response")
}

func blockForToolRiskReview(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, record *toolbroker.CallRecord, cause error) error {
	if session == nil || task == nil || record == nil {
		if cause != nil {
			return cause
		}
		return fmt.Errorf("tool risk review required")
	}
	causeText := ""
	if cause != nil {
		causeText = cause.Error()
	}
	body := "A critical-risk tool call was blocked before execution. Review the request and choose whether to retry with safer inputs, skip it, or stop the run."
	if err := services.Runs.UpdateSessionStatus(ctx, session.RunID, runpkg.SessionStatusBlocked); err != nil {
		return err
	}
	if err := services.Runs.SetTaskBlocked(ctx, task.TaskID, runpkg.TaskStatusBlocked, "tool_risk_review:"+record.ToolCallID); err != nil {
		return err
	}
	if services.Blocked != nil {
		_, _ = services.Blocked.Create(ctx, blockedpkg.CreateRequest{
			SessionID:          session.SessionID,
			RunID:              session.RunID,
			TaskID:             task.TaskID,
			Kind:               blockedpkg.KindToolRiskReview,
			Title:              "Review critical tool request",
			Body:               body,
			RequiredAction:     "retry_skip_or_stop",
			ResumeTargetTaskID: task.TaskID,
			ToolCallID:         record.ToolCallID,
			Metadata: rawJSON(map[string]any{
				"tool_id": record.ToolID,
				"risk":    record.Risk,
				"error":   sharedcontext.RedactText(causeText),
			}),
		})
	}
	_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, trace.EventTaskBlocked, task.AgentID, map[string]any{
		"session_id":   session.SessionID,
		"task_id":      task.TaskID,
		"reason":       "tool_risk_review",
		"tool_call_id": record.ToolCallID,
		"tool_id":      record.ToolID,
		"risk":         record.Risk,
	})
	return fmt.Errorf("tool_risk_review: %s", body)
}

func executePendingModelToolCalls(ctx context.Context, options Options, services Services, session *runpkg.Session, task *runpkg.Task, raw string) ([]string, error) {
	var calls []modelToolCall
	if err := json.Unmarshal([]byte(raw), &calls); err != nil {
		return nil, fmt.Errorf("resume tool loop: decode pending tool calls: %w", err)
	}
	if services.Tools == nil {
		return nil, fmt.Errorf("resume tool loop: tool broker is not initialized")
	}
	broker := newToolBroker(options, services)
	observations := []string{}
	for _, call := range calls {
		toolID := normalizeModelToolID(call.ToolID)
		if toolID == "" {
			observations = append(observations, fmt.Sprintf("Tool %q was skipped because it is not available.", call.ToolID))
			continue
		}
		record, err := broker.Execute(ctx, toolbroker.ExecuteRequest{
			SessionID: session.SessionID,
			RunID:     session.RunID,
			TaskID:    task.TaskID,
			AgentID:   task.AgentID,
			ToolID:    toolID,
			Input:     call.Input,
		})
		if record != nil && record.Status == toolbroker.StatusWaitingApproval {
			pendingJSON, _ := json.Marshal([]modelToolCall{call})
			_ = updateTaskMetadata(ctx, services.Runs, task, map[string]any{
				"tool_loop_pending_json": string(pendingJSON),
			})
			return observations, blockForToolApproval(ctx, services, session, task, record)
		}
		observations = append(observations, toolObservation(toolID, record, err))
		if err != nil && record == nil {
			return observations, err
		}
	}
	return observations, nil
}

func extractModelToolCalls(output string) ([]modelToolCall, bool) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, false
	}
	candidates := []string{output}
	if fenced := fencedJSON(output); fenced != "" {
		candidates = append([]string{fenced}, candidates...)
	}
	if object := firstJSONObject(output); object != "" && object != output {
		candidates = append(candidates, object)
	}
	for _, candidate := range candidates {
		var envelope modelToolEnvelope
		if err := json.Unmarshal([]byte(candidate), &envelope); err != nil {
			continue
		}
		calls := make([]modelToolCall, 0, len(envelope.ToolCalls))
		for _, call := range envelope.ToolCalls {
			if strings.TrimSpace(call.ToolID) == "" {
				continue
			}
			if call.Input == nil {
				call.Input = map[string]any{}
			}
			calls = append(calls, call)
		}
		if len(calls) > 0 {
			return calls, true
		}
	}
	return nil, false
}

func modelToolCallsFromResult(result *runpkg.Result) ([]modelToolCall, bool) {
	if result == nil {
		return nil, false
	}
	if len(result.ToolCalls) > 0 {
		calls := make([]modelToolCall, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			toolID := strings.TrimSpace(call.ToolID)
			if toolID == "" {
				continue
			}
			input := call.Input
			if input == nil {
				input = map[string]any{}
			}
			calls = append(calls, modelToolCall{ID: call.ID, ToolID: toolID, Input: input})
		}
		if len(calls) > 0 {
			return calls, true
		}
	}
	return extractModelToolCalls(resultPreview(result))
}

func modelToolSchemas() []adapters.ToolSchema {
	definitions := toolbroker.Definitions()
	schemas := make([]adapters.ToolSchema, 0, len(definitions))
	for _, definition := range definitions {
		schemas = append(schemas, adapters.ToolSchema{
			ID:          definition.ID,
			Description: definition.Description,
			Parameters:  definition.Parameters,
		})
	}
	return schemas
}

func fencedJSON(output string) string {
	start := strings.Index(output, "```")
	if start < 0 {
		return ""
	}
	rest := output[start+3:]
	if strings.HasPrefix(strings.ToLower(rest), "json") {
		rest = rest[4:]
	}
	end := strings.Index(rest, "```")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func firstJSONObject(output string) string {
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(output[start : end+1])
}

func normalizeModelToolID(toolID string) string {
	switch strings.TrimSpace(toolID) {
	case toolbroker.ToolListFiles, "read_project", "list_project":
		return toolbroker.ToolListFiles
	case toolbroker.ToolReadFile:
		return toolbroker.ToolReadFile
	case toolbroker.ToolWriteFile, "write_project":
		return toolbroker.ToolWriteFile
	case toolbroker.ToolReplaceFile, "str_replace", "edit_file":
		return toolbroker.ToolReplaceFile
	case toolbroker.ToolPresentArtifact, "artifact":
		return toolbroker.ToolPresentArtifact
	case toolbroker.ToolBash, "run_checks", "shell":
		return toolbroker.ToolBash
	case toolbroker.ToolSearch, "web_search":
		return toolbroker.ToolSearch
	case toolbroker.ToolFetch, "web_fetch":
		return toolbroker.ToolFetch
	default:
		return ""
	}
}

func modelToolFailureSignature(toolID string, input map[string]any) string {
	payload, err := json.Marshal(input)
	if err != nil {
		return toolID
	}
	return toolID + ":" + string(payload)
}

func redactedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, sharedcontext.RedactText(value))
	}
	return result
}

func appendToolObservations(prompt string, observations []string) string {
	if len(observations) == 0 {
		return prompt
	}
	var builder strings.Builder
	builder.WriteString(prompt)
	builder.WriteString("\n\nTool observations:\n")
	for _, observation := range observations {
		builder.WriteString("- ")
		builder.WriteString(strings.ReplaceAll(strings.TrimSpace(observation), "\n", "\n  "))
		builder.WriteString("\n")
	}
	builder.WriteString("\nContinue this role. Return another tool_calls JSON object only if more tools are required; otherwise return the final role result as normal text.")
	return builder.String()
}

func toolObservation(toolID string, record *toolbroker.CallRecord, err error) string {
	if record == nil {
		if err != nil {
			return fmt.Sprintf("%s failed: %s", toolID, err.Error())
		}
		return toolID + " produced no record."
	}
	if err != nil {
		if record.OutputPreview != "" {
			return fmt.Sprintf("%s failed: %s\n%s", toolID, err.Error(), record.OutputPreview)
		}
		return fmt.Sprintf("%s failed: %s", toolID, err.Error())
	}
	if record.OutputPreview == "" {
		return fmt.Sprintf("%s completed.", toolID)
	}
	return fmt.Sprintf("%s output:\n%s", toolID, record.OutputPreview)
}

func toolCallID(record *toolbroker.CallRecord) string {
	if record == nil {
		return ""
	}
	return record.ToolCallID
}

func toolCallStatus(record *toolbroker.CallRecord, err error) string {
	if record != nil && record.Status != "" {
		return record.Status
	}
	if err != nil {
		return toolbroker.StatusFailed
	}
	return toolbroker.StatusCompleted
}

func stopIfSessionRunnable(ctx context.Context, services Services, sessionID string) error {
	detail, err := services.Runs.GetBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	switch detail.Session.Status {
	case runpkg.SessionStatusRunning:
		return nil
	case runpkg.SessionStatusCancelled:
		return fmt.Errorf("session cancelled")
	case runpkg.SessionStatusPlanReview, runpkg.SessionStatusBlocked, runpkg.SessionStatusNeedsClarification:
		return fmt.Errorf("session paused: %s", detail.Session.Status)
	default:
		return fmt.Errorf("session is %s", detail.Session.Status)
	}
}

func stopIfSessionCancelled(ctx context.Context, services Services, sessionID string) error {
	detail, err := services.Runs.GetBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	if detail.Session.Status == runpkg.SessionStatusCancelled {
		return fmt.Errorf("session cancelled")
	}
	return nil
}

func routeNeedsPlanReview(session *runpkg.Session) bool {
	if session == nil || len(session.Metadata) == 0 {
		return false
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(session.Metadata, &metadata); err != nil {
		return false
	}
	if value, ok := metadata["needs_plan_review"].(bool); ok {
		return value
	}
	route, _ := metadata["route_decision"].(map[string]any)
	value, _ := route["needs_plan_review"].(bool)
	return value
}

func rolePrompt(ctx context.Context, configPath string, services Services, session *runpkg.Session, originalPrompt string, task *runpkg.Task, summaries []string) string {
	var builder strings.Builder
	builder.WriteString(originalPrompt)
	builder.WriteString("\n\nRole task:\n")
	builder.WriteString(task.AgentID)
	if purpose := taskMetadataString(task, "purpose"); purpose != "" {
		builder.WriteString("\nPurpose: ")
		builder.WriteString(purpose)
	}
	skillBriefings := skills.Briefings(configPath, taskMetadataStringSlice(task, "required_skills"))
	if len(skillBriefings) > 0 {
		builder.WriteString("\n\nSelected skill briefings:\n")
		for _, briefing := range skillBriefings {
			builder.WriteString("- ")
			builder.WriteString(briefing)
			builder.WriteString("\n")
		}
	}
	if memory := reusableMemoryBriefings(ctx, services.Context, session); len(memory) > 0 {
		builder.WriteString("\n\nApproved reusable context:\n")
		for _, item := range memory {
			builder.WriteString("- ")
			builder.WriteString(item.Text)
			builder.WriteString("\n")
		}
		_ = appendRunLedgerTrace(ctx, services.Trace, session.RunID, "memory.context.loaded", task.AgentID, map[string]any{
			"task_id":     task.TaskID,
			"context_ids": memoryContextIDs(memory),
		})
	}
	if len(summaries) > 0 {
		builder.WriteString("\n\nPrior role summaries:\n")
		for _, summary := range summaries {
			builder.WriteString("- ")
			builder.WriteString(summary)
			builder.WriteString("\n")
		}
	}
	if shouldOfferToolProtocol(task) {
		builder.WriteString("\n\nTool protocol:\n")
		builder.WriteString("Use the provided native tool calls when the provider supports them. If the provider does not expose native tools, return only compact JSON in this fallback shape:\n")
		builder.WriteString(`{"tool_calls":[{"tool_id":"read_file","input":{"path":"README.md","max_bytes":12000}}]}`)
		builder.WriteString("\nAvailable tool_id values: list_files, read_file, write_file, replace_file, present_artifact, bash, search, fetch.\n")
		builder.WriteString("Use normal prose only when you are done with tool use. Mutating tools can pause for user approval and then resume from saved state.\n")
	}
	builder.WriteString("\nReturn a concise task result with decisions, evidence, and next handoff notes.")
	return builder.String()
}

type reusableMemoryBriefing struct {
	ContextID string
	Text      string
}

func reusableMemoryBriefings(ctx context.Context, store *sharedcontext.Store, session *runpkg.Session) []reusableMemoryBriefing {
	if store == nil || session == nil || session.ProjectID == "" {
		return nil
	}
	items, err := store.ListItems(ctx, session.ProjectID, sharedcontext.ScopeProject, 5)
	if err != nil {
		return nil
	}
	briefings := []reusableMemoryBriefing{}
	for _, item := range items {
		body := strings.Join(strings.Fields(item.Body), " ")
		if body == "" {
			continue
		}
		if len(body) > 500 {
			body = body[:497] + "..."
		}
		briefings = append(briefings, reusableMemoryBriefing{ContextID: item.ContextID, Text: item.Title + ": " + body})
	}
	return briefings
}

func memoryContextIDs(items []reusableMemoryBriefing) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.ContextID != "" {
			ids = append(ids, item.ContextID)
		}
	}
	return ids
}

func shouldOfferToolProtocol(task *runpkg.Task) bool {
	if task == nil {
		return false
	}
	if len(taskMetadataStringSlice(task, "required_tools")) > 0 {
		return true
	}
	switch taskRoleID(task) {
	case "researcher", "coder":
		return true
	default:
		return false
	}
}

func taskSummary(task *runpkg.Task, preview string) string {
	if preview == "" {
		return task.AgentID + " completed without text output."
	}
	if len(preview) > 600 {
		return preview[:600]
	}
	return preview
}

func resultPreview(result *runpkg.Result) string {
	if result == nil {
		return ""
	}
	if len(result.Messages) > 0 {
		return result.Messages[len(result.Messages)-1].Content
	}
	if result.CLI != nil {
		if result.CLI.Stdout != "" {
			return result.CLI.Stdout
		}
		return result.CLI.Stderr
	}
	return ""
}

func createRoleArtifact(ctx context.Context, services Services, session *runpkg.Session, task *runpkg.Task, preview string, finalRole bool) error {
	if services.Artifacts == nil {
		return nil
	}
	artifactType := ""
	title := ""
	reviewState := artifactpkg.ReviewDraft
	switch {
	case taskRoleID(task) == "planner":
		artifactType = artifactpkg.TypePlan
		title = "Plan"
	case finalRole:
		artifactType = artifactpkg.TypeReport
		title = "Final report"
		reviewState = artifactpkg.ReviewApproved
	default:
		return nil
	}
	path := ""
	if services.Sandboxes != nil {
		if record, err := services.Sandboxes.GetByRun(ctx, session.RunID); err == nil && record.ArtifactRoot != "" {
			if err := os.MkdirAll(record.ArtifactRoot, 0o700); err != nil {
				return err
			}
			filename := strings.ReplaceAll(strings.ToLower(title), " ", "-") + ".md"
			path = filepath.Join(record.ArtifactRoot, filename)
			if err := os.WriteFile(path, []byte(preview), 0o600); err != nil {
				return err
			}
		}
	}
	artifact, err := services.Artifacts.Create(ctx, artifactpkg.CreateRequest{
		SessionID:   session.SessionID,
		RunID:       session.RunID,
		TaskID:      task.TaskID,
		Type:        artifactType,
		Title:       title,
		Path:        path,
		ReviewState: reviewState,
		Preview:     preview,
	})
	if err != nil {
		return err
	}
	if err := services.Runs.AddTaskArtifactRef(ctx, task.TaskID, artifact.ArtifactID); err != nil {
		return err
	}
	return appendRunLedgerTrace(ctx, services.Trace, session.RunID, trace.EventArtifactCreated, task.AgentID, map[string]any{
		"artifact_id":  artifact.ArtifactID,
		"task_id":      task.TaskID,
		"type":         artifact.Type,
		"title":        artifact.Title,
		"review_state": artifact.ReviewState,
	})
}

func updateTaskMetadata(ctx context.Context, store *runpkg.Store, task *runpkg.Task, updates map[string]any) error {
	metadata := map[string]any{}
	if len(task.Metadata) > 0 {
		_ = json.Unmarshal(task.Metadata, &metadata)
	}
	for key, value := range updates {
		metadata[key] = value
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	task.Metadata = payload
	return store.UpdateTaskMetadata(ctx, task.TaskID, payload)
}

func taskMetadataString(task *runpkg.Task, key string) string {
	metadata := map[string]any{}
	if len(task.Metadata) == 0 {
		return ""
	}
	if err := json.Unmarshal(task.Metadata, &metadata); err != nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

func taskMetadataStringSlice(task *runpkg.Task, key string) []string {
	metadata := map[string]any{}
	if len(task.Metadata) == 0 {
		return nil
	}
	if err := json.Unmarshal(task.Metadata, &metadata); err != nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return typed
	case []any:
		values := []string{}
		for _, value := range typed {
			text, ok := value.(string)
			if ok && strings.TrimSpace(text) != "" {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func rawJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func taskRoleID(task *runpkg.Task) string {
	if roleID := taskMetadataString(task, "role_id"); roleID != "" {
		return roleID
	}
	return task.AgentID
}

func taskIDs(tasks []*runpkg.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
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
	if services.Sandboxes != nil {
		if err := services.Sandboxes.ReleaseByRun(ctx, runID); err != nil {
			return err
		}
		if err := appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventSandboxReleased, "", map[string]any{
			"status": "released",
		}); err != nil {
			return err
		}
	}
	return appendRunLedgerTrace(ctx, services.Trace, runID, trace.EventRunSessionCompleted, "", map[string]any{
		"status": sessionStatus,
	})
}

func sandboxIntentFromConfig(configPath string) (sandbox.Intent, error) {
	if configPath == "" {
		configPath = "nomici.yaml"
	}
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil {
		return sandbox.Intent{}, err
	}
	if loaded.Spec == nil {
		return sandbox.Intent{}, fmt.Errorf("AgentSpec is empty")
	}
	return sandbox.ParseIntent(loaded.Spec.Deployment)
}

func sandboxBaseDir(configPath string) (string, error) {
	if configPath == "" {
		configPath = "nomici.yaml"
	}
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absolute), nil
}

type ledgerTaskPlan struct {
	AgentID   string
	RuntimeID string
	Metadata  map[string]any
}

func ledgerTaskPlans(ctx context.Context, services Services, snapshot *graph.Snapshot, startAgentID string, routeDecision *orchestration.RouteDecision) ([]ledgerTaskPlan, error) {
	if routeDecision == nil {
		decision := orchestration.Route("", startAgentID, snapshot)
		decision.Goal = ""
		routeDecision = &decision
	}
	if plans, err := packRoleTaskPlans(ctx, services, snapshot, startAgentID, routeDecision); err != nil {
		return nil, err
	} else if len(plans) > 0 {
		return plans, nil
	}
	return graphHandoffTaskPlans(snapshot, startAgentID, routeDecision), nil
}

func packRoleTaskPlans(ctx context.Context, services Services, snapshot *graph.Snapshot, startAgentID string, routeDecision *orchestration.RouteDecision) ([]ledgerTaskPlan, error) {
	if services.Packs == nil {
		return nil, nil
	}
	installations, err := services.Packs.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	for _, installation := range installations {
		manifest, ok := packs.GetBuiltin(installation.PackID)
		if !ok || !containsString(installation.Entrypoints, startAgentID) || len(manifest.Roles) == 0 {
			continue
		}
		match := orchestration.MatchRoles(manifest, snapshot, startAgentID, *routeDecision)
		if routeDecision != nil {
			routeDecision.SelectedRoles = routeDecision.SelectedRoles[:0]
			for _, role := range match.Roles {
				routeDecision.SelectedRoles = append(routeDecision.SelectedRoles, role.RoleID)
			}
			routeDecision.RequiredTools = match.RequiredTools
			routeDecision.NeedsPlanReview = match.NeedsReview
		}
		plans := make([]ledgerTaskPlan, 0, len(match.Roles))
		for _, role := range match.Roles {
			agent, ok := snapshot.IR.Agents[role.AgentID]
			if !ok {
				plans = nil
				break
			}
			metadata := roleSelectionMetadata(manifest, role, match, routeDecision)
			plans = append(plans, ledgerTaskPlan{
				AgentID:   agent.ID,
				RuntimeID: agent.Runtime,
				Metadata:  metadata,
			})
		}
		if len(plans) > 0 {
			return plans, nil
		}
	}
	return nil, nil
}

func roleSelectionMetadata(manifest packs.Manifest, role orchestration.RoleSelection, match orchestration.MatchResult, routeDecision *orchestration.RouteDecision) map[string]any {
	return map[string]any{
		"plan_source":          "pack_role",
		"pack_id":              manifest.ID,
		"pack_version":         manifest.Version,
		"sequence":             role.Sequence,
		"role_id":              role.RoleID,
		"purpose":              role.Purpose,
		"required_tools":       role.RequiredTools,
		"required_skills":      role.RequiredSkills,
		"handoff_mode":         "sequential",
		"output_contract":      role.OutputContract,
		"route_decision":       routeDecision,
		"match_score":          role.MatchScore,
		"selection_reason":     role.SelectionReason,
		"skipped_roles":        match.SkippedRoles,
		"match_required_tools": match.RequiredTools,
	}
}

func graphHandoffTaskPlans(snapshot *graph.Snapshot, startAgentID string, routeDecision *orchestration.RouteDecision) []ledgerTaskPlan {
	plans := []ledgerTaskPlan{}
	visited := map[string]bool{}
	current := startAgentID
	sequence := 1
	for current != "" && !visited[current] {
		visited[current] = true
		agent, ok := snapshot.IR.Agents[current]
		if !ok {
			break
		}
		plans = append(plans, ledgerTaskPlan{
			AgentID:   agent.ID,
			RuntimeID: agent.Runtime,
			Metadata: map[string]any{
				"plan_source":    "graph_handoff",
				"sequence":       sequence,
				"role_id":        agent.ID,
				"route_decision": routeDecision,
			},
		})
		sequence++
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
		plans = append(plans, ledgerTaskPlan{
			AgentID: startAgentID,
			Metadata: map[string]any{
				"plan_source":    "graph_handoff",
				"sequence":       1,
				"role_id":        startAgentID,
				"route_decision": routeDecision,
			},
		})
	}
	return plans
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
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
		if services.Blocked != nil {
			_ = services.Blocked.ResolveByApproval(request.Context(), approval.ApprovalID, blockedpkg.StatusResolved)
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
		if services.Blocked != nil {
			_ = services.Blocked.ResolveByApproval(request.Context(), approval.ApprovalID, blockedpkg.StatusRejected)
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
