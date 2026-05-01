package gateway

import (
	"context"
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
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

func TestConsoleOverviewEndpoint(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	providerStore := providers.NewStore(db)
	if err := providerStore.Save(context.Background(), &providers.Profile{
		ID:        "gpt",
		Name:      "gpt",
		Kind:      providers.KindOpenAICompatible,
		BaseURL:   "http://127.0.0.1:19999/v1",
		Model:     "fake-model",
		APIKeyEnv: "NOMICI_TEST_API_KEY",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	packStore := packs.NewStore(db)
	if err := packStore.SaveInstallation(context.Background(), &packs.Installation{
		PackID:      packs.DeveloperTeamID,
		Version:     "0.1.0",
		Kind:        "agent_pack",
		Trust:       "official",
		ConfigPath:  filepath.Join(dir, "nomici.yaml"),
		Entrypoints: []string{"product_pm"},
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("save pack installation: %v", err)
	}

	graphStore := graph.NewStore(db)
	if err := graphStore.Save(context.Background(), &graph.Snapshot{
		SnapshotID:    "graph_test",
		SchemaVersion: "0.1",
		ProjectID:     "test-project",
		CreatedAt:     time.Now().UTC(),
		SourceHash:    "sha256:test",
		IR: graph.IR{
			Models: map[string]graph.Model{
				"gpt": {ID: "gpt", Kind: providers.KindOpenAICompatible, Model: "fake-model"},
			},
			Runtimes: map[string]graph.Runtime{
				"implementer_cli": {ID: "implementer_cli", Kind: "cli_agent", Workspace: "./workspace", Trust: "untrusted"},
			},
			Agents: map[string]graph.Agent{
				"product_pm":  {ID: "product_pm", Kind: "gateway_agent", Model: "gpt"},
				"implementer": {ID: "implementer", Kind: "external_agent", Runtime: "implementer_cli"},
			},
			Edges: []graph.Edge{{ID: "edge_1", From: "product_pm", To: "implementer", Mode: "handoff"}},
		},
	}); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	traceStore := trace.NewStore(db)
	if err := traceStore.Append(context.Background(), &trace.Event{
		RunID: "run_test",
		Type:  trace.EventRunCompleted,
	}); err != nil {
		t.Fatalf("append trace: %v", err)
	}

	policyService := policy.NewService(db)
	if _, err := policyService.Check(context.Background(), policy.ActionRequest{
		RunID:      "run_approval",
		ProjectID:  "test-project",
		AgentID:    "implementer",
		RuntimeID:  "implementer_cli",
		Workspace:  filepath.Join(dir, "workspace"),
		FilesWrite: true,
	}); err != nil {
		t.Fatalf("create pending approval: %v", err)
	}

	router := NewRouter(Options{Version: "test"}, Services{
		Providers: providerStore,
		Trace:     traceStore,
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewOpenAICompatibleAdapter(),
		Graph:     graphStore,
		Packs:     packStore,
		Policy:    policyService,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/console/overview", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sk-test-secret") {
		t.Fatal("console overview leaked raw secret")
	}

	var envelope struct {
		Data consoleOverview `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if envelope.Data.Counts.Models != 1 {
		t.Fatalf("expected one model, got %d", envelope.Data.Counts.Models)
	}
	if envelope.Data.Counts.PacksInstalled != 1 {
		t.Fatalf("expected one installed pack, got %d", envelope.Data.Counts.PacksInstalled)
	}
	if envelope.Data.Graph == nil || envelope.Data.Graph.AgentCount != 2 {
		t.Fatalf("unexpected graph summary: %+v", envelope.Data.Graph)
	}
	if len(envelope.Data.Runtimes) != 1 || envelope.Data.Runtimes[0].Status != "configured" {
		t.Fatalf("unexpected runtimes: %+v", envelope.Data.Runtimes)
	}
	if len(envelope.Data.LatestTrace) != 1 || envelope.Data.LatestTrace[0].Type != trace.EventRunCompleted {
		t.Fatalf("unexpected latest trace: %+v", envelope.Data.LatestTrace)
	}
	if len(envelope.Data.PendingApprovals) != 1 {
		t.Fatalf("expected one pending approval, got %d", len(envelope.Data.PendingApprovals))
	}
}
