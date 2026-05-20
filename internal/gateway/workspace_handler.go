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

	artifactpkg "github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
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
