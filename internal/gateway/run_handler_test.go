package gateway

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
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
	events := waitForRunEvents(t, trace.NewStore(db), envelope.Data.RunID, trace.EventRunCompleted)
	if len(events) != 8 {
		t.Fatalf("expected eight trace events, got %d", len(events))
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
	if len(eventsEnvelope.Data) != 6 {
		t.Fatalf("expected six events after sequence 2, got %d", len(eventsEnvelope.Data))
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
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	providerStore := providers.NewStore(db)
	traceStore := trace.NewStore(db)
	return db, NewRouter(Options{Version: "test", ConfigPath: "nomici.yaml"}, Services{
		Providers: providerStore,
		Trace:     traceStore,
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewOpenAICompatibleAdapter(),
		Graph:     graph.NewStore(db),
		Packs:     packs.NewStore(db),
		Policy:    policy.NewService(db),
		Context:   sharedcontext.NewStore(db),
		Runs:      runpkg.NewStore(db),
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
