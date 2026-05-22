package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	artifactpkg "github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	blockedpkg "github.com/NomiciAI/nomici-orchestrator/internal/blocked"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	uploadpkg "github.com/NomiciAI/nomici-orchestrator/internal/uploads"
	"github.com/go-chi/chi/v5"
)

const maxUploadBytes = 25 * 1024 * 1024

func sessionResumeHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session services are not initialized.", "Restart Gateway.")
			return
		}
		sessionID := chi.URLParam(request, "session_id")
		detail, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		if detail.Session.Status == runpkg.SessionStatusPlanReview {
			if services.Artifacts == nil {
				writeError(response, http.StatusServiceUnavailable, requestID, "plan_review_unavailable", "Plan review services are not initialized.", "Restart Gateway.")
				return
			}
			artifactID, err := latestPlanArtifactID(request.Context(), services, detail.Session.SessionID)
			if err != nil {
				writeError(response, http.StatusConflict, requestID, "plan_not_approved", "Plan review must be approved before resume.", "Approve the plan first.")
				return
			}
			artifact, err := services.Artifacts.Get(request.Context(), artifactID)
			if err != nil || artifact.ReviewState != artifactpkg.ReviewApproved {
				writeError(response, http.StatusConflict, requestID, "plan_not_approved", "Plan review must be approved before resume.", "Approve the plan first.")
				return
			}
		}
		if err := services.Runs.ResumeSession(request.Context(), sessionID); err != nil {
			writeError(response, http.StatusConflict, requestID, "session_not_resumable", err.Error(), "Only blocked, plan review, or clarification sessions can be resumed.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventRunSessionCreated, "", map[string]any{
			"session_id": sessionID,
			"status":     runpkg.SessionStatusRunning,
			"reason":     "resume",
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_resume_trace_failed", "Run session resume could not be traced.", "Check Gateway logs.")
			return
		}
		updated, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		if startErr := resumeWorkspaceWorker(request.Context(), options, services, updated); startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		payload, err := sessionDetailPayload(request.Context(), services, updated)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionPlanReviseHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Artifacts == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "plan_review_unavailable", "Plan review services are not initialized.", "Restart Gateway.")
			return
		}
		var body struct {
			ArtifactID string `json:"artifact_id"`
			Plan       string `json:"plan"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send artifact_id and plan.")
			return
		}
		if strings.TrimSpace(body.Plan) == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "plan is required.", "Send revised plan text.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		artifactID := body.ArtifactID
		if artifactID == "" {
			artifactID, err = latestPlanArtifactID(request.Context(), services, detail.Session.SessionID)
			if err != nil {
				writeError(response, http.StatusNotFound, requestID, "plan_not_found", "No plan artifact was found for this session.", "Wait for planner output or refresh the session.")
				return
			}
		}
		artifact, err := services.Artifacts.Revise(request.Context(), artifactID, body.Plan)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "plan_revise_failed", err.Error(), "Refresh the plan and retry.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventArtifactCreated, "", map[string]any{
			"artifact_id":  artifact.ArtifactID,
			"type":         artifact.Type,
			"revision":     artifact.Revision,
			"review_state": artifact.ReviewState,
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "plan_revise_trace_failed", "Plan revision could not be traced.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, artifact, nil)
	}
}

func sessionPlanApproveHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Artifacts == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "plan_review_unavailable", "Plan review services are not initialized.", "Restart Gateway.")
			return
		}
		var body struct {
			ArtifactID string `json:"artifact_id"`
		}
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		artifactID := body.ArtifactID
		if artifactID == "" {
			artifactID, err = latestPlanArtifactID(request.Context(), services, detail.Session.SessionID)
			if err != nil {
				writeError(response, http.StatusNotFound, requestID, "plan_not_found", "No plan artifact was found for this session.", "Wait for planner output or refresh the session.")
				return
			}
		}
		artifact, err := services.Artifacts.SetReviewState(request.Context(), artifactID, artifactpkg.ReviewApproved)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "plan_approve_failed", err.Error(), "Refresh the plan and retry.")
			return
		}
		if services.Blocked != nil {
			_ = services.Blocked.ResolveByArtifact(request.Context(), artifact.ArtifactID)
		}
		if err := services.Runs.ResumeSession(request.Context(), detail.Session.SessionID); err != nil {
			writeError(response, http.StatusConflict, requestID, "session_not_resumable", err.Error(), "Only plan review sessions can be approved.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventArtifactCreated, "", map[string]any{
			"artifact_id":  artifact.ArtifactID,
			"type":         artifact.Type,
			"revision":     artifact.Revision,
			"review_state": artifact.ReviewState,
			"reason":       "plan_approved",
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "plan_approve_trace_failed", "Plan approval could not be traced.", "Check Gateway logs.")
			return
		}
		updated, err := services.Runs.GetBySession(request.Context(), detail.Session.SessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		if startErr := resumeWorkspaceWorker(request.Context(), options, services, updated); startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		payload, err := sessionDetailPayload(request.Context(), services, updated)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionBlockedActionsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Blocked == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "blocked_actions_unavailable", "Blocked action store is not initialized.", "Restart Gateway.")
			return
		}
		status := request.URL.Query().Get("status")
		actions, err := services.Blocked.ListBySession(request.Context(), chi.URLParam(request, "session_id"), status, 50)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "blocked_actions_failed", "Blocked actions could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, actions, nil)
	}
}

func sessionApprovalsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Policy == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "approvals_unavailable", "Run approval services are not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		approvals, err := services.Policy.ListByRun(request.Context(), detail.Session.RunID, request.URL.Query().Get("status"))
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "approvals_failed", "Session approvals could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, approvals, nil)
	}
}

type sessionContextUsageItem struct {
	EventID    string   `json:"event_id"`
	TaskID     string   `json:"task_id,omitempty"`
	AgentID    string   `json:"agent_id,omitempty"`
	ContextIDs []string `json:"context_ids,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Time       string   `json:"time"`
}

func sessionContextUsageHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "context_usage_unavailable", "Run context services are not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		events, err := services.Trace.ListByRun(request.Context(), detail.Session.RunID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "context_usage_failed", "Context usage could not be loaded.", "Check Gateway logs.")
			return
		}
		items := []sessionContextUsageItem{}
		for _, event := range events {
			if event.Type != "memory.context.loaded" {
				continue
			}
			var payload struct {
				TaskID     string   `json:"task_id"`
				AgentID    string   `json:"agent_id"`
				ContextIDs []string `json:"context_ids"`
				Summary    string   `json:"summary"`
			}
			_ = json.Unmarshal(event.Payload, &payload)
			items = append(items, sessionContextUsageItem{
				EventID:    event.EventID,
				TaskID:     payload.TaskID,
				AgentID:    payload.AgentID,
				ContextIDs: payload.ContextIDs,
				Summary:    payload.Summary,
				Time:       event.Time.Format(time.RFC3339Nano),
			})
		}
		writeSuccess(response, requestID, items, nil)
	}
}

func reviewQueueHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Blocked == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "review_queue_unavailable", "Review queue store is not initialized.", "Restart Gateway.")
			return
		}
		status := request.URL.Query().Get("status")
		if status == "" {
			status = blockedpkg.StatusOpen
		}
		limit := 50
		if raw := request.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "limit must be between 1 and 100.", "Use a positive integer limit.")
				return
			}
			limit = parsed
		}
		actions, err := services.Blocked.List(request.Context(), status, limit)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "review_queue_failed", "Review queue could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, actions, nil)
	}
}

func sessionBlockedActionResolveHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Blocked == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "blocked_actions_unavailable", "Blocked action services are not initialized.", "Restart Gateway.")
			return
		}
		sessionID := chi.URLParam(request, "session_id")
		actionID := chi.URLParam(request, "blocked_action_id")
		var body struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
			Resume   *bool  `json:"resume"`
		}
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		detail, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		metadata := rawJSON(map[string]string{
			"decision": strings.TrimSpace(body.Decision),
			"note":     strings.TrimSpace(body.Note),
		})
		var action *blockedpkg.Action
		if strings.EqualFold(body.Decision, "reject") || strings.EqualFold(body.Decision, "stop") {
			action, err = services.Blocked.Reject(request.Context(), actionID, metadata)
		} else {
			action, err = services.Blocked.Resolve(request.Context(), actionID, metadata)
		}
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "blocked_action_resolve_failed", err.Error(), "Refresh the workspace and retry.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventTaskBlocked, "", map[string]any{
			"session_id":        sessionID,
			"blocked_action_id": action.BlockedActionID,
			"kind":              action.Kind,
			"reason":            "blocked_action_resolved",
			"decision":          body.Decision,
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "blocked_action_trace_failed", "Blocked action trace event could not be written.", "Check Gateway logs.")
			return
		}
		switch strings.ToLower(strings.TrimSpace(body.Decision)) {
		case "skip":
			if action.ResumeTargetTaskID != "" {
				for _, task := range detail.Tasks {
					if task.TaskID == action.ResumeTargetTaskID {
						_ = updateTaskMetadata(request.Context(), services.Runs, task, map[string]any{
							"summary": "Skipped after user resolved blocked action.",
						})
						break
					}
				}
				_ = services.Runs.UpdateTaskStatus(request.Context(), action.ResumeTargetTaskID, runpkg.TaskStatusCompleted)
			}
		case "stop":
			_ = services.Runs.CancelSession(request.Context(), sessionID)
			_ = services.Runs.CancelTasks(request.Context(), detail.Session.RunID)
			updated, lookupErr := services.Runs.GetBySession(request.Context(), sessionID)
			if lookupErr == nil {
				detail = updated
			}
		}
		shouldResume := action.Status == blockedpkg.StatusResolved
		if body.Resume != nil {
			shouldResume = *body.Resume && shouldResume
		}
		if shouldResume {
			if err := services.Runs.ResumeSession(request.Context(), sessionID); err == nil {
				if updated, lookupErr := services.Runs.GetBySession(request.Context(), sessionID); lookupErr == nil {
					detail = updated
					if startErr := resumeWorkspaceWorker(request.Context(), options, services, updated); startErr != nil {
						writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
						return
					}
				}
			}
		}
		payload, err := sessionDetailPayload(request.Context(), services, detail)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func sessionClarificationHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Blocked == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "clarifications_unavailable", "Clarification services are not initialized.", "Restart Gateway.")
			return
		}
		var body struct {
			BlockedActionID string `json:"blocked_action_id"`
			Answer          string `json:"answer"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send blocked_action_id and answer.")
			return
		}
		body.Answer = strings.TrimSpace(body.Answer)
		if body.Answer == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "answer is required.", "Send the missing information.")
			return
		}
		sessionID := chi.URLParam(request, "session_id")
		detail, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		actionID := body.BlockedActionID
		if actionID == "" {
			actions, err := services.Blocked.ListBySession(request.Context(), sessionID, blockedpkg.StatusOpen, 20)
			if err != nil {
				writeError(response, http.StatusInternalServerError, requestID, "blocked_actions_failed", "Blocked actions could not be loaded.", "Check Gateway logs.")
				return
			}
			for _, action := range actions {
				if action.Kind == blockedpkg.KindClarification {
					actionID = action.BlockedActionID
					break
				}
			}
		}
		if actionID == "" {
			writeError(response, http.StatusNotFound, requestID, "clarification_not_found", "No open clarification was found for this session.", "Refresh the workspace.")
			return
		}
		action, err := services.Blocked.Resolve(request.Context(), actionID, rawJSON(map[string]string{"answer": body.Answer}))
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "clarification_resolve_failed", err.Error(), "Refresh the workspace and retry.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventTaskBlocked, "", map[string]any{
			"session_id":        sessionID,
			"blocked_action_id": action.BlockedActionID,
			"kind":              blockedpkg.KindClarification,
			"reason":            "clarification_answered",
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "clarification_trace_failed", "Clarification trace event could not be written.", "Check Gateway logs.")
			return
		}
		if err := services.Runs.ResumeSession(request.Context(), sessionID); err != nil {
			writeError(response, http.StatusConflict, requestID, "session_not_resumable", err.Error(), "Only blocked or clarification sessions can be resumed.")
			return
		}
		updated, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		if startErr := resumeWorkspaceWorker(request.Context(), options, services, updated); startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		payload, err := sessionDetailPayload(request.Context(), services, updated)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, payload, nil)
	}
}

func uploadListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Uploads == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "uploads_unavailable", "Upload store is not initialized.", "Restart Gateway.")
			return
		}
		limit := 50
		if raw := request.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "limit must be between 1 and 100.", "Use a positive integer limit.")
				return
			}
			limit = parsed
		}
		uploads, err := services.Uploads.List(request.Context(), request.URL.Query().Get("session_id"), limit)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "uploads_list_failed", "Uploads could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, uploads, nil)
	}
}

func uploadCreateHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil || services.Sandboxes == nil || services.Uploads == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "uploads_unavailable", "Upload services are not initialized.", "Restart Gateway.")
			return
		}
		if err := request.ParseMultipartForm(maxUploadBytes); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_upload", "Upload must be multipart form data.", "Send session_id and file.")
			return
		}
		sessionID := request.FormValue("session_id")
		if sessionID == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_upload", "session_id is required.", "Send session_id with the upload.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), sessionID)
		if err != nil {
			writeSessionLookupError(response, requestID, err)
			return
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_upload", "file is required.", "Attach a file field.")
			return
		}
		defer file.Close()
		if header.Size > maxUploadBytes {
			writeError(response, http.StatusRequestEntityTooLarge, requestID, "upload_too_large", "Upload exceeds the maximum size.", "Use a file up to 25 MiB.")
			return
		}
		filename, err := safeFilename(header.Filename)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_upload_path", err.Error(), "Use a simple file name.")
			return
		}
		sandboxRecord, err := services.Sandboxes.GetByRun(request.Context(), detail.Session.RunID)
		if err != nil {
			writeError(response, http.StatusConflict, requestID, "workspace_unavailable", "Workspace is not available for this session.", "Start a session before uploading files.")
			return
		}
		uploadRoot := uploadRootForSandbox(sandboxRecord.WorkspaceRoot)
		if err := os.MkdirAll(uploadRoot, 0o700); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "upload_store_failed", "Upload directory could not be created.", "Check Gateway logs.")
			return
		}
		target := filepath.Join(uploadRoot, filename)
		if err := writeUploadedFile(target, file); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "upload_store_failed", "Upload could not be stored.", "Check Gateway logs.")
			return
		}
		upload, err := services.Uploads.Create(request.Context(), uploadpkg.CreateRequest{
			SessionID:   detail.Session.SessionID,
			RunID:       detail.Session.RunID,
			Filename:    filename,
			Path:        target,
			SizeBytes:   header.Size,
			ContentType: header.Header.Get("Content-Type"),
		})
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "upload_record_failed", "Upload record could not be stored.", "Check Gateway logs.")
			return
		}
		if err := appendRunLedgerTrace(request.Context(), services.Trace, detail.Session.RunID, trace.EventUploadCreated, "", map[string]any{
			"upload_id": upload.UploadID,
			"filename":  upload.Filename,
			"size":      upload.SizeBytes,
		}); err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "upload_trace_failed", "Upload could not be traced.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, upload, nil)
	}
}

func artifactListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Artifacts == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "artifacts_unavailable", "Artifact store is not initialized.", "Restart Gateway.")
			return
		}
		artifacts, err := services.Artifacts.List(request.Context(), request.URL.Query().Get("session_id"), 50)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "artifacts_list_failed", "Artifacts could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, artifacts, nil)
	}
}

func artifactDetailHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Artifacts == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "artifacts_unavailable", "Artifact store is not initialized.", "Restart Gateway.")
			return
		}
		artifact, err := services.Artifacts.Get(request.Context(), chi.URLParam(request, "artifact_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "artifact_not_found", "Artifact was not found.", "Refresh artifacts.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "artifact_load_failed", "Artifact could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, artifact, nil)
	}
}

func artifactRevisionsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Artifacts == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "artifacts_unavailable", "Artifact store is not initialized.", "Restart Gateway.")
			return
		}
		revisions, err := services.Artifacts.ListRevisions(request.Context(), chi.URLParam(request, "artifact_id"), 50)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "artifact_revisions_failed", "Artifact revisions could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, revisions, nil)
	}
}

func artifactContentHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Artifacts == nil || services.Sandboxes == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "artifacts_unavailable", "Artifact services are not initialized.", "Restart Gateway.")
			return
		}
		artifact, err := services.Artifacts.Get(request.Context(), chi.URLParam(request, "artifact_id"))
		if err != nil {
			writeArtifactLookupError(response, requestID, err)
			return
		}
		path, err := readableArtifactPath(request.Context(), services, artifact)
		if err != nil {
			writeError(response, http.StatusConflict, requestID, "artifact_content_unavailable", err.Error(), "Use artifact preview or regenerate the artifact.")
			return
		}
		file, err := os.Open(path)
		if err != nil {
			writeError(response, http.StatusNotFound, requestID, "artifact_file_not_found", "Artifact file was not found on disk.", "Refresh artifacts or inspect the run workspace.")
			return
		}
		defer file.Close()
		payload, err := io.ReadAll(io.LimitReader(file, 1<<20))
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "artifact_read_failed", "Artifact file could not be read.", "Check Gateway logs.")
			return
		}
		truncated := false
		if stat, statErr := file.Stat(); statErr == nil && stat.Size() > int64(len(payload)) {
			truncated = true
		}
		writeSuccess(response, requestID, map[string]any{
			"artifact_id": artifact.ArtifactID,
			"path":        artifact.Path,
			"content":     string(payload),
			"truncated":   truncated,
		}, nil)
	}
}

func artifactDownloadHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Artifacts == nil || services.Sandboxes == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "artifacts_unavailable", "Artifact services are not initialized.", "Restart Gateway.")
			return
		}
		artifact, err := services.Artifacts.Get(request.Context(), chi.URLParam(request, "artifact_id"))
		if err != nil {
			writeArtifactLookupError(response, requestID, err)
			return
		}
		path, err := readableArtifactPath(request.Context(), services, artifact)
		if err != nil {
			writeError(response, http.StatusConflict, requestID, "artifact_download_unavailable", err.Error(), "Use artifact preview or regenerate the artifact.")
			return
		}
		filename := filepath.Base(path)
		if filename == "." || filename == string(filepath.Separator) || filename == "" {
			filename = artifact.ArtifactID + ".txt"
		}
		response.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
		http.ServeFile(response, request, path)
	}
}

func writeArtifactLookupError(response http.ResponseWriter, requestID string, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(response, http.StatusNotFound, requestID, "artifact_not_found", "Artifact was not found.", "Refresh artifacts.")
		return
	}
	writeError(response, http.StatusInternalServerError, requestID, "artifact_load_failed", "Artifact could not be loaded.", "Check Gateway logs.")
}

func readableArtifactPath(ctx context.Context, services Services, artifact *artifactpkg.Artifact) (string, error) {
	if artifact == nil || strings.TrimSpace(artifact.Path) == "" {
		return "", fmt.Errorf("artifact has no file path")
	}
	if !filepath.IsAbs(artifact.Path) {
		return "", fmt.Errorf("artifact path is not absolute")
	}
	sandboxRecord, err := services.Sandboxes.GetByRun(ctx, artifact.RunID)
	if err != nil {
		return "", fmt.Errorf("workspace is not available for this artifact")
	}
	path, err := filepath.Abs(artifact.Path)
	if err != nil {
		return "", err
	}
	roots := []string{sandboxRecord.WorkspaceRoot, sandboxRecord.ArtifactRoot}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if path == absRoot || strings.HasPrefix(path, absRoot+string(filepath.Separator)) {
			return path, nil
		}
	}
	return "", fmt.Errorf("artifact path is outside the session workspace")
}

func latestPlanArtifactID(ctx context.Context, services Services, sessionID string) (string, error) {
	artifacts, err := services.Artifacts.List(ctx, sessionID, 20)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		if artifact.Type == artifactpkg.TypePlan {
			return artifact.ArtifactID, nil
		}
	}
	return "", sql.ErrNoRows
}

func safeFilename(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", fmt.Errorf("filename is required")
	}
	if strings.Contains(base, "..") || strings.ContainsAny(base, `/\`) {
		return "", fmt.Errorf("filename must not contain path segments")
	}
	return base, nil
}

func uploadRootForSandbox(workspaceRoot string) string {
	if workspaceRoot == "" {
		return filepath.Join(".nomici", "uploads")
	}
	return filepath.Join(filepath.Dir(workspaceRoot), "uploads")
}

func writeUploadedFile(path string, source io.Reader) error {
	target, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, io.LimitReader(source, maxUploadBytes+1))
	return err
}

func writeSessionLookupError(response http.ResponseWriter, requestID string, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
		return
	}
	writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
}
