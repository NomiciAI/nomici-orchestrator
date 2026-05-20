package gateway

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	"github.com/NomiciAI/nomici-orchestrator/internal/blocked"
	"github.com/NomiciAI/nomici-orchestrator/internal/chats"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/memory"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/NomiciAI/nomici-orchestrator/internal/uploads"
	"github.com/NomiciAI/nomici-orchestrator/internal/worklocks"
)

func TestRunCreateEndpointValidation(t *testing.T) {
	db, router := newRunTestRouter(t)
	graphStore := graph.NewStore(db)
	saveRunTestGraph(t, graphStore, "http://127.0.0.1:1", []graph.Edge{})

	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "missing prompt", body: `{"agent_id":"product_pm"}`, status: http.StatusBadRequest, code: "run_not_supported"},
		{name: "unknown agent", body: `{"agent_id":"missing","prompt":"hello"}`, status: http.StatusBadRequest, code: "run_not_supported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("expected error code %q, got %s", test.code, response.Body.String())
			}
		})
	}
}

func TestRunCreateEndpointMissingGraph(t *testing.T) {
	_, router := newRunTestRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "graph_not_found") {
		t.Fatalf("expected graph_not_found, got %s", response.Body.String())
	}
}

func TestRunCreateEndpointUnsupportedGraphEdge(t *testing.T) {
	db, router := newRunTestRouter(t)
	saveRunTestGraph(t, graph.NewStore(db), "http://127.0.0.1:1", []graph.Edge{
		{ID: "edge_1", From: "product_pm", To: "architect", Mode: "handoff"},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "run_not_supported") {
		t.Fatalf("expected run_not_supported, got %s", response.Body.String())
	}
}

func TestRunCreateEndpointModelRunAndEvents(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer sk-test-secret" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Console run completed."}}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouter(t)
	saveRunTestGraph(t, graph.NewStore(db), providerServer.URL, []graph.Edge{})

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sk-test-secret") {
		t.Fatal("run create response leaked raw secret")
	}
	var envelope struct {
		Data runCreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode run create: %v", err)
	}
	events := waitForRunEvents(t, trace.NewStore(db), envelope.Data.RunID, trace.EventRunSessionCompleted)
	if len(events) < 13 {
		t.Fatalf("expected session trace events, got %d", len(events))
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/runs/"+envelope.Data.RunID+"/events?after_sequence=2", nil)
	eventsResponse := httptest.NewRecorder()
	router.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", eventsResponse.Code, eventsResponse.Body.String())
	}
	if strings.Contains(eventsResponse.Body.String(), "sk-test-secret") {
		t.Fatal("events response leaked raw secret")
	}
	if !strings.Contains(eventsResponse.Body.String(), "Console run completed.") {
		t.Fatalf("expected assistant output in events response, got %s", eventsResponse.Body.String())
	}
	var eventsEnvelope struct {
		Data []traceEventResponse `json:"data"`
	}
	if err := json.NewDecoder(eventsResponse.Body).Decode(&eventsEnvelope); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(eventsEnvelope.Data) < 11 {
		t.Fatalf("expected events after sequence 2, got %d", len(eventsEnvelope.Data))
	}
	if eventsEnvelope.Data[0].Sequence <= 2 {
		t.Fatalf("expected sequence filtering, got %+v", eventsEnvelope.Data)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/runs/"+envelope.Data.RunID, nil)
	detailResponse := httptest.NewRecorder()
	router.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailResponse.Code, detailResponse.Body.String())
	}
	if !strings.Contains(detailResponse.Body.String(), `"status":"completed"`) || !strings.Contains(detailResponse.Body.String(), `"agent_id":"product_pm"`) {
		t.Fatalf("expected completed session detail, got %s", detailResponse.Body.String())
	}
	if !strings.Contains(detailResponse.Body.String(), `"sandbox_id":"sandbox_`) || !strings.Contains(detailResponse.Body.String(), `"cleanup_status":"released"`) {
		t.Fatalf("expected sandbox detail, got %s", detailResponse.Body.String())
	}
	var detailEnvelope struct {
		Data struct {
			Sandbox *sandbox.Record `json:"sandbox"`
		} `json:"data"`
	}
	if err := json.NewDecoder(strings.NewReader(detailResponse.Body.String())).Decode(&detailEnvelope); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detailEnvelope.Data.Sandbox == nil || !filepath.IsAbs(detailEnvelope.Data.Sandbox.WorkspaceRoot) || !strings.Contains(detailEnvelope.Data.Sandbox.WorkspaceRoot, filepath.Join(".nomici", "runs")) {
		t.Fatalf("expected absolute sandbox root anchored at config directory, got %+v", detailEnvelope.Data.Sandbox)
	}

	sessionsRequest := httptest.NewRequest(http.MethodGet, "/api/sessions?limit=1", nil)
	sessionsResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionsResponse, sessionsRequest)
	if sessionsResponse.Code != http.StatusOK {
		t.Fatalf("expected sessions status 200, got %d: %s", sessionsResponse.Code, sessionsResponse.Body.String())
	}
	if !strings.Contains(sessionsResponse.Body.String(), envelope.Data.SessionID) {
		t.Fatalf("expected session in session list, got %s", sessionsResponse.Body.String())
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/sessions/"+envelope.Data.SessionID, nil)
	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("expected session detail status 200, got %d: %s", sessionResponse.Code, sessionResponse.Body.String())
	}
	if !strings.Contains(sessionResponse.Body.String(), `"session_id":"`+envelope.Data.SessionID+`"`) || !strings.Contains(sessionResponse.Body.String(), `"tasks"`) {
		t.Fatalf("expected session detail with tasks, got %s", sessionResponse.Body.String())
	}

	tasksRequest := httptest.NewRequest(http.MethodGet, "/api/sessions/"+envelope.Data.SessionID+"/tasks", nil)
	tasksResponse := httptest.NewRecorder()
	router.ServeHTTP(tasksResponse, tasksRequest)
	if tasksResponse.Code != http.StatusOK {
		t.Fatalf("expected tasks status 200, got %d: %s", tasksResponse.Code, tasksResponse.Body.String())
	}
	if !strings.Contains(tasksResponse.Body.String(), `"agent_id":"product_pm"`) {
		t.Fatalf("expected task list, got %s", tasksResponse.Body.String())
	}
}

func TestRunCreateEndpointModelToolLoop(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	var calls int64
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if atomic.AddInt64(&calls, 1) == 1 {
			_, _ = response.Write([]byte(`{
				"choices": [{"message": {"role": "assistant", "content": "{\"tool_calls\":[{\"tool_id\":\"list_files\",\"input\":{\"path\":\".\",\"limit\":5}}]}"}}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
			}`))
			return
		}
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Final answer after inspecting workspace."}}],
			"usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouter(t)
	saveRunTestGraph(t, graph.NewStore(db), providerServer.URL, []graph.Edge{})

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"inspect workspace files"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data runCreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode run create: %v", err)
	}
	waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusCompleted)
	if got := atomic.LoadInt64(&calls); got < 2 {
		t.Fatalf("expected model to be reinvoked after tool observation, got %d call(s)", got)
	}
	records, err := toolbroker.NewStore(db).ListBySession(context.Background(), envelope.Data.SessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ToolID != toolbroker.ToolListFiles || records[0].Status != toolbroker.StatusCompleted {
		t.Fatalf("expected completed list_files call, got %+v", records)
	}
	detail, err := runpkg.NewStore(db).GetBySession(context.Background(), envelope.Data.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary := taskMetadataString(detail.Tasks[0], "summary"); !strings.Contains(summary, "Final answer") {
		t.Fatalf("expected final model output in task summary, got %q", summary)
	}
}

func TestModelToolLoopRepeatedFailureCreatesRetryDecision(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "{\"tool_calls\":[{\"tool_id\":\"read_file\",\"input\":{\"path\":\"missing.txt\"}}]}"}}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouter(t)
	saveRunTestGraph(t, graph.NewStore(db), providerServer.URL, []graph.Edge{})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"inspect missing file"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data runCreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode run create: %v", err)
	}
	detail := waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusBlocked)
	if detail.Tasks[0].BlockedReason != "repeated_tool_failure" {
		t.Fatalf("expected repeated failure block, got %+v", detail.Tasks[0])
	}
	actions, err := blocked.NewStore(db).ListBySession(context.Background(), envelope.Data.SessionID, blocked.StatusOpen, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Kind != blocked.KindRetryDecision {
		t.Fatalf("expected retry decision action, got %+v", actions)
	}
}

func TestModelToolLoopApprovalResumesPendingCall(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	var calls int64
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if atomic.AddInt64(&calls, 1) == 1 {
			_, _ = response.Write([]byte(`{
				"choices": [{"message": {"role": "assistant", "content": "{\"tool_calls\":[{\"tool_id\":\"write_file\",\"input\":{\"path\":\"notes/result.txt\",\"content\":\"approved write\"}}]}"}}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
			}`))
			return
		}
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Final answer after approved write."}}],
			"usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouterWithConfig(t, `version: "0.1"
project:
  name: test
deployment:
  sandbox:
    mode: local
    file_write_enabled: true
`)
	saveRunTestGraph(t, graph.NewStore(db), providerServer.URL, []graph.Edge{})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"write a workspace note"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data runCreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode run create: %v", err)
	}
	waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusBlocked)
	actions, err := blocked.NewStore(db).ListBySession(context.Background(), envelope.Data.SessionID, blocked.StatusOpen, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Kind != blocked.KindToolApproval || actions[0].ApprovalID == "" {
		t.Fatalf("expected open tool approval blocked action, got %+v", actions)
	}
	grant := httptest.NewRecorder()
	router.ServeHTTP(grant, httptest.NewRequest(http.MethodPost, "/api/approvals/"+actions[0].ApprovalID+"/grant", bytes.NewBufferString(`{"scope":"run"}`)))
	if grant.Code != http.StatusOK {
		t.Fatalf("expected grant 200, got %d: %s", grant.Code, grant.Body.String())
	}
	resume := httptest.NewRecorder()
	router.ServeHTTP(resume, httptest.NewRequest(http.MethodPost, "/api/sessions/"+envelope.Data.SessionID+"/resume", nil))
	if resume.Code != http.StatusOK {
		t.Fatalf("expected resume 200, got %d: %s", resume.Code, resume.Body.String())
	}
	completed := waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusCompleted)
	_ = completed
	sandboxRecord, err := sandbox.NewStore(db).GetByRun(context.Background(), envelope.Data.RunID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(sandboxRecord.WorkspaceRoot, "notes", "result.txt"))
	if err != nil {
		t.Fatalf("expected approved write to create file: %v", err)
	}
	if string(payload) != "approved write" {
		t.Fatalf("expected approved write content, got %q", string(payload))
	}
	resolved, err := blocked.NewStore(db).ListBySession(context.Background(), envelope.Data.SessionID, blocked.StatusResolved, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) == 0 {
		t.Fatal("expected blocked action to resolve after approval")
	}
}

func TestRunCreateEndpointRolePlanReviewAndArtifacts(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Role output with plan and report evidence."}}],
			"usage": {"prompt_tokens": 7, "completion_tokens": 5, "total_tokens": 12}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouter(t)
	manifest := packs.DeveloperTeamManifest()
	if err := packs.NewStore(db).SaveInstallation(context.Background(), &packs.Installation{
		PackID:      manifest.ID,
		Version:     manifest.Version,
		Kind:        manifest.Kind,
		Trust:       manifest.Trust.Level,
		ConfigPath:  "nomici.yaml",
		Entrypoints: manifest.Agents.Entrypoints,
		InstalledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	saveRunTestRoleGraph(t, graph.NewStore(db), providerServer.URL)

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"ship workspace"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data runCreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode run create: %v", err)
	}
	detail := waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusPlanReview)
	if len(detail.Tasks) != 4 {
		t.Fatalf("expected dynamic role task plan, got %+v", detail.Tasks)
	}
	if detail.Tasks[2].AgentID != "coder" || detail.Tasks[3].AgentID != "reporter" {
		t.Fatalf("expected implementation route to include coder and reporter, got %+v", detail.Tasks)
	}
	planArtifacts, err := artifacts.NewStore(db).List(context.Background(), envelope.Data.SessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(planArtifacts) != 1 || planArtifacts[0].Type != artifacts.TypePlan {
		t.Fatalf("expected plan artifact, got %+v", planArtifacts)
	}

	approve := httptest.NewRecorder()
	router.ServeHTTP(approve, httptest.NewRequest(http.MethodPost, "/api/sessions/"+envelope.Data.SessionID+"/plan/approve", nil))
	if approve.Code != http.StatusOK {
		t.Fatalf("expected plan approve 200, got %d: %s", approve.Code, approve.Body.String())
	}
	waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusBlocked)
	pendingApprovals, err := policy.NewService(db).List(context.Background(), policy.StatusPending)
	if err != nil {
		t.Fatal(err)
	}
	if len(pendingApprovals) == 0 {
		t.Fatal("expected pending tool approval")
	}
	for _, approval := range pendingApprovals {
		grant := httptest.NewRecorder()
		router.ServeHTTP(grant, httptest.NewRequest(http.MethodPost, "/api/approvals/"+approval.ApprovalID+"/grant", bytes.NewBufferString(`{"scope":"run"}`)))
		if grant.Code != http.StatusOK {
			t.Fatalf("expected grant approval 200, got %d: %s", grant.Code, grant.Body.String())
		}
	}
	resume := httptest.NewRecorder()
	router.ServeHTTP(resume, httptest.NewRequest(http.MethodPost, "/api/sessions/"+envelope.Data.SessionID+"/resume", nil))
	if resume.Code != http.StatusOK {
		t.Fatalf("expected resume 200, got %d: %s", resume.Code, resume.Body.String())
	}
	completed := waitForSessionStatus(t, runpkg.NewStore(db), envelope.Data.SessionID, runpkg.SessionStatusCompleted)
	for _, task := range completed.Tasks {
		if task.Status != runpkg.TaskStatusCompleted {
			t.Fatalf("expected completed role task, got %+v", completed.Tasks)
		}
	}
	finalArtifacts, err := artifacts.NewStore(db).List(context.Background(), envelope.Data.SessionID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(finalArtifacts) < 2 {
		t.Fatalf("expected plan and final report artifacts, got %+v", finalArtifacts)
	}
}

func TestSessionEndpointsValidationAndCancel(t *testing.T) {
	db, router := newRunTestRouter(t)
	runStore := runpkg.NewStore(db)
	session, err := runStore.CreateSession(context.Background(), runpkg.CreateSessionRequest{
		RunID:           "run_session_api",
		ProjectID:       "project",
		GraphSnapshotID: "graph",
		Title:           "Session API task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStore.CreateTask(context.Background(), runpkg.CreateTaskRequest{
		RunID:   "run_session_api",
		AgentID: "planner",
		Status:  runpkg.TaskStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}

	badLimit := httptest.NewRecorder()
	router.ServeHTTP(badLimit, httptest.NewRequest(http.MethodGet, "/api/sessions?limit=0", nil))
	if badLimit.Code != http.StatusBadRequest {
		t.Fatalf("expected bad limit 400, got %d: %s", badLimit.Code, badLimit.Body.String())
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/sessions/session_missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected missing session 404, got %d: %s", missing.Code, missing.Body.String())
	}

	cancelResponse := httptest.NewRecorder()
	router.ServeHTTP(cancelResponse, httptest.NewRequest(http.MethodPost, "/api/sessions/"+session.SessionID+"/cancel", nil))
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d: %s", cancelResponse.Code, cancelResponse.Body.String())
	}
	if !strings.Contains(cancelResponse.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("expected cancelled session response, got %s", cancelResponse.Body.String())
	}
	events, err := trace.NewStore(db).ListByRun(context.Background(), "run_session_api")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != trace.EventRunSessionCompleted {
		t.Fatalf("expected cancellation trace event, got %+v", events)
	}
	secondCancel := httptest.NewRecorder()
	router.ServeHTTP(secondCancel, httptest.NewRequest(http.MethodPost, "/api/sessions/"+session.SessionID+"/cancel", nil))
	if secondCancel.Code != http.StatusConflict {
		t.Fatalf("expected terminal cancel conflict, got %d: %s", secondCancel.Code, secondCancel.Body.String())
	}
}

func TestChatMessageCreatesRunSession(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Chat run completed."}}],
			"usage": {"prompt_tokens": 4, "completion_tokens": 3, "total_tokens": 7}
		}`))
	}))
	defer providerServer.Close()

	db, router := newRunTestRouter(t)
	saveRunTestGraph(t, graph.NewStore(db), providerServer.URL, []graph.Edge{})

	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/api/chats", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello from chat"}`)))
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected chat create 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var envelope struct {
		Data chatMessageResponse `json:"data"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Message.ChatID == "" || envelope.Data.Run == nil || envelope.Data.Run.SessionID == "" {
		t.Fatalf("expected chat message and run metadata, got %+v", envelope.Data)
	}
	waitForRunEvents(t, trace.NewStore(db), envelope.Data.Run.RunID, trace.EventRunCompleted)

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/chats", nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), envelope.Data.Message.ChatID) {
		t.Fatalf("expected chat in list, got %d: %s", listResponse.Code, listResponse.Body.String())
	}

	detailResponse := httptest.NewRecorder()
	router.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/api/chats/"+envelope.Data.Message.ChatID, nil))
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), "hello from chat") {
		t.Fatalf("expected chat detail, got %d: %s", detailResponse.Code, detailResponse.Body.String())
	}
}

func TestChatDirectReplyDoesNotCreateRun(t *testing.T) {
	db, router := newRunTestRouter(t)
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/api/chats", bytes.NewBufferString(`{"prompt":"setup status"}`)))
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected chat create 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var envelope struct {
		Data chatMessageResponse `json:"data"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Run != nil {
		t.Fatalf("expected direct reply without run, got %+v", envelope.Data.Run)
	}
	if envelope.Data.RouteDecision == nil || envelope.Data.RouteDecision.Mode != orchestration.ModeDirectReply {
		t.Fatalf("expected direct route decision, got %+v", envelope.Data.RouteDecision)
	}
	if envelope.Data.AssistantMessage == nil || envelope.Data.AssistantMessage.Role != chats.RoleAssistant {
		t.Fatalf("expected assistant message, got %+v", envelope.Data.AssistantMessage)
	}
	sessions, err := runpkg.NewStore(db).ListSessions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no run sessions, got %+v", sessions)
	}
}

func TestLedgerTaskPlansUseInstalledPackRoles(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	manifest := packs.DeveloperTeamManifest()
	if err := packs.NewStore(db).SaveInstallation(context.Background(), &packs.Installation{
		PackID:      manifest.ID,
		Version:     manifest.Version,
		Kind:        manifest.Kind,
		Trust:       manifest.Trust.Level,
		ConfigPath:  "nomici.yaml",
		Entrypoints: manifest.Agents.Entrypoints,
		InstalledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := &graph.Snapshot{
		SnapshotID: "graph_roles",
		ProjectID:  "test-project",
		IR: graph.IR{Agents: map[string]graph.Agent{
			"product_pm": {ID: "product_pm", Kind: "gateway_agent", Model: "gpt"},
			"planner":    {ID: "planner", Kind: "model_agent", Model: "gpt"},
			"researcher": {ID: "researcher", Kind: "model_agent", Model: "gpt"},
			"coder":      {ID: "coder", Kind: "model_agent", Model: "gpt"},
			"reporter":   {ID: "reporter", Kind: "model_agent", Model: "gpt"},
		}},
	}
	decision := orchestration.Route("implement workspace artifacts", "", snapshot)
	plans, err := ledgerTaskPlans(context.Background(), Services{Packs: packs.NewStore(db)}, snapshot, "product_pm", &decision)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 4 {
		t.Fatalf("expected dynamic implementation role plans, got %+v", plans)
	}
	if plans[0].AgentID != "product_pm" || plans[3].AgentID != "reporter" {
		t.Fatalf("expected pack role order, got %+v", plans)
	}
	if plans[2].Metadata["plan_source"] != "pack_role" || plans[2].Metadata["role_id"] != "coder" {
		t.Fatalf("expected coder role metadata, got %+v", plans[2].Metadata)
	}
	outputContract, ok := plans[2].Metadata["output_contract"].(packs.OutputContract)
	if !ok || outputContract.Kind != "implementation_result" {
		t.Fatalf("expected coder output contract, got %+v", plans[2].Metadata["output_contract"])
	}
}

func TestRunCreateFailsOnInvalidSandboxConfig(t *testing.T) {
	db, router := newRunTestRouterWithConfig(t, `version: "0.1"
project:
  name: test
deployment:
  sandbox: invalid
`)
	saveRunTestGraph(t, graph.NewStore(db), "http://127.0.0.1:1", []graph.Edge{})

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "sandbox_config_invalid") {
		t.Fatalf("expected sandbox config error, got %s", response.Body.String())
	}
	sessions, err := runpkg.NewStore(db).ListSessions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no partial run ledger, got %+v", sessions)
	}
}

func TestRunCreateFailsWhenSandboxUnavailable(t *testing.T) {
	previousDetect := detectSandboxAvailability
	detectSandboxAvailability = func(mode string) sandbox.Availability {
		return sandbox.Availability{
			Provider: sandbox.ProviderContainerRuntime,
			Mode:     sandbox.ModeContainer,
			Status:   sandbox.StatusUnavailable,
			Message:  "container runtime missing",
		}
	}
	t.Cleanup(func() { detectSandboxAvailability = previousDetect })

	db, router := newRunTestRouterWithConfig(t, `version: "0.1"
project:
  name: test
deployment:
  sandbox:
    mode: container
`)
	saveRunTestGraph(t, graph.NewStore(db), "http://127.0.0.1:1", []graph.Edge{})

	request := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(`{"agent_id":"product_pm","prompt":"hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "sandbox_unavailable") {
		t.Fatalf("expected sandbox unavailable error, got %s", response.Body.String())
	}
	sessions, err := runpkg.NewStore(db).ListSessions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no partial run ledger, got %+v", sessions)
	}
}

func TestApprovalMutationEndpoints(t *testing.T) {
	db, router := newRunTestRouter(t)
	policyService := policy.NewService(db)
	decision, err := policyService.Check(context.Background(), policy.ActionRequest{
		RunID:      "run_approval",
		ProjectID:  "test-project",
		AgentID:    "implementer",
		RuntimeID:  "implementer_cli",
		Workspace:  filepath.Join(t.TempDir(), "workspace"),
		FilesWrite: true,
	})
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}
	if decision.ApprovalID == "" {
		t.Fatal("expected pending approval")
	}

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/approvals/"+decision.ApprovalID+"/grant", bytes.NewBufferString(`{"scope":"run"}`))
	grantResponse := httptest.NewRecorder()
	router.ServeHTTP(grantResponse, grantRequest)
	if grantResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", grantResponse.Code, grantResponse.Body.String())
	}
	if !strings.Contains(grantResponse.Body.String(), `"status":"granted"`) {
		t.Fatalf("expected granted approval, got %s", grantResponse.Body.String())
	}

	secondGrant := httptest.NewRecorder()
	router.ServeHTTP(secondGrant, httptest.NewRequest(http.MethodPost, "/api/approvals/"+decision.ApprovalID+"/grant", nil))
	if secondGrant.Code != http.StatusBadRequest {
		t.Fatalf("expected non-pending grant to fail, got %d: %s", secondGrant.Code, secondGrant.Body.String())
	}

	events, err := trace.NewStore(db).ListByRun(context.Background(), "run_approval")
	if err != nil {
		t.Fatalf("list approval trace: %v", err)
	}
	if len(events) != 1 || events[0].Type != trace.EventApprovalGranted {
		t.Fatalf("expected approval.granted trace, got %+v", events)
	}

	denyDecision, err := policyService.Check(context.Background(), policy.ActionRequest{
		RunID:      "run_deny",
		ProjectID:  "test-project",
		AgentID:    "implementer",
		RuntimeID:  "implementer_cli",
		Workspace:  filepath.Join(t.TempDir(), "workspace"),
		FilesWrite: true,
		Summary:    "different action",
	})
	if err != nil {
		t.Fatalf("create deny approval: %v", err)
	}
	denyResponse := httptest.NewRecorder()
	router.ServeHTTP(denyResponse, httptest.NewRequest(http.MethodPost, "/api/approvals/"+denyDecision.ApprovalID+"/deny", nil))
	if denyResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", denyResponse.Code, denyResponse.Body.String())
	}
	if !strings.Contains(denyResponse.Body.String(), `"status":"denied"`) {
		t.Fatalf("expected denied approval, got %s", denyResponse.Body.String())
	}
}

func newRunTestRouter(t *testing.T) (*sql.DB, http.Handler) {
	t.Helper()
	return newRunTestRouterWithConfig(t, `version: "0.1"
project:
  name: test
deployment:
  sandbox:
    mode: local
`)
}

func newRunTestRouterWithConfig(t *testing.T, config string) (*sql.DB, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nomici.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	db, err := store.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	providerStore := providers.NewStore(db)
	traceStore := trace.NewStore(db)
	return db, NewRouter(Options{Version: "test", ConfigPath: configPath}, Services{
		Providers: providerStore,
		Trace:     traceStore,
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewModelAdapter(),
		Graph:     graph.NewStore(db),
		Packs:     packs.NewStore(db),
		Policy:    policy.NewService(db),
		Context:   sharedcontext.NewStore(db),
		Runs:      runpkg.NewStore(db),
		Sandboxes: sandbox.NewStore(db),
		Artifacts: artifacts.NewStore(db),
		Uploads:   uploads.NewStore(db),
		Chats:     chats.NewStore(db),
		Tools:     toolbroker.NewStore(db),
		Memory:    memory.NewStore(db),
		Blocked:   blocked.NewStore(db),
		Locks:     worklocks.NewStore(db),
	})
}

func saveRunTestGraph(t *testing.T, store *graph.Store, baseURL string, edges []graph.Edge) {
	t.Helper()
	if err := store.Save(context.Background(), &graph.Snapshot{
		SnapshotID:    "graph_test",
		SchemaVersion: "0.1",
		ProjectID:     "test-project",
		CreatedAt:     time.Now().UTC(),
		SourceHash:    "sha256:test",
		IR: graph.IR{
			Models: map[string]graph.Model{
				"gpt": {
					ID:        "gpt",
					Kind:      providers.KindOpenAICompatible,
					BaseURL:   baseURL,
					Model:     "test-model",
					APIKeyEnv: "NOMICI_TEST_API_KEY",
				},
			},
			Runtimes: map[string]graph.Runtime{},
			Agents: map[string]graph.Agent{
				"product_pm": {ID: "product_pm", Kind: "gateway_agent", Model: "gpt"},
				"architect":  {ID: "architect", Kind: "gateway_agent", Model: "gpt"},
			},
			Edges: edges,
		},
	}); err != nil {
		t.Fatalf("save graph: %v", err)
	}
}

func saveRunTestRoleGraph(t *testing.T, store *graph.Store, baseURL string) {
	t.Helper()
	agents := map[string]graph.Agent{}
	for _, id := range []string{"product_pm", "planner", "researcher", "coder", "reporter"} {
		kind := "model_agent"
		if id == "product_pm" {
			kind = "gateway_agent"
		}
		agents[id] = graph.Agent{ID: id, Kind: kind, Model: "gpt"}
	}
	if err := store.Save(context.Background(), &graph.Snapshot{
		SnapshotID:    "graph_roles",
		SchemaVersion: "0.1",
		ProjectID:     "test-project",
		CreatedAt:     time.Now().UTC(),
		SourceHash:    "sha256:test",
		IR: graph.IR{
			Models: map[string]graph.Model{
				"gpt": {
					ID:        "gpt",
					Kind:      providers.KindOpenAICompatible,
					BaseURL:   baseURL,
					Model:     "test-model",
					APIKeyEnv: "NOMICI_TEST_API_KEY",
				},
			},
			Runtimes: map[string]graph.Runtime{},
			Agents:   agents,
			Edges:    []graph.Edge{},
		},
	}); err != nil {
		t.Fatalf("save graph: %v", err)
	}
}

func waitForRunEvents(t *testing.T, store *trace.Store, runID string, terminalType string) []*trace.Event {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		events, err := store.ListByRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("list run events: %v", err)
		}
		for _, event := range events {
			if event.Type == terminalType {
				return events
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach %s", runID, terminalType)
	return nil
}

func waitForSessionStatus(t *testing.T, store *runpkg.Store, sessionID string, status string) *runpkg.SessionDetail {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last *runpkg.SessionDetail
	for time.Now().Before(deadline) {
		detail, err := store.GetBySession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		last = detail
		if detail.Session.Status == status {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	if last != nil {
		taskState := []string{}
		for _, task := range last.Tasks {
			taskState = append(taskState, fmt.Sprintf("%s:%s:%s:%s", task.TaskID, task.AgentID, task.Status, taskMetadataString(task, "failure_reason")))
		}
		t.Fatalf("session %s did not reach %s; last status %s tasks %v", sessionID, status, last.Session.Status, taskState)
	}
	t.Fatalf("session %s did not reach %s", sessionID, status)
	return nil
}
