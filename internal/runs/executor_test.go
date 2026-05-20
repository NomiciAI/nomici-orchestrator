package runs

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

func TestExecuteCLIHandoffChain(t *testing.T) {
	db, configPath := newExecutorTestDB(t)
	script := writeExecutorTestScript(t, `
#!/bin/sh
printf "%s handled:\n%s\n" "$LABEL" "$1"
`)
	workspace := t.TempDir()
	snapshot := executorTestChainSnapshot(workspace, script, []graph.Edge{
		{ID: "edge_1", From: "planner", To: "implementer", Mode: "handoff"},
		{ID: "edge_2", From: "implementer", To: "reviewer", Mode: "handoff"},
	})

	executor := DBExecutor(db, adapters.NewModelAdapter(), secrets.NewResolver(), configPath)
	result, err := executor.Execute(context.Background(), Request{
		Snapshot: snapshot,
		AgentID:  "planner",
		Prompt:   "ship a small feature",
		RunID:    "run_chain",
	})
	if err != nil {
		t.Fatalf("execute handoff chain: %v", err)
	}
	if result.Status != clirunner.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.RuntimeID != "reviewer_runtime" {
		t.Fatalf("expected final runtime, got %s", result.RuntimeID)
	}
	if result.ContextSnapshotID == "" {
		t.Fatal("expected handoff context snapshot id")
	}
	if result.CLI == nil || !strings.Contains(result.CLI.Stdout, "reviewer handled") {
		t.Fatalf("expected reviewer stdout, got %+v", result.CLI)
	}
	if !strings.Contains(result.CLI.Stdout, "Task briefing:") || !strings.Contains(result.CLI.Stdout, "implementer handled") {
		t.Fatalf("expected downstream shared context in final stdout, got %q", result.CLI.Stdout)
	}

	events, err := tracepkg.NewStore(db).ListByRun(context.Background(), "run_chain")
	if err != nil {
		t.Fatalf("list trace: %v", err)
	}
	assertTraceCount(t, events, tracepkg.EventHandoffCreated, 2)
	assertTraceCount(t, events, tracepkg.EventHandoffContextAttached, 2)
	assertTraceCount(t, events, tracepkg.EventHandoffAccepted, 2)
	assertTraceCount(t, events, tracepkg.EventRunCompleted, 1)

	snapshots, err := sharedcontext.NewStore(db).ListSnapshots(context.Background(), "test-project", 10)
	if err != nil {
		t.Fatalf("list context snapshots: %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("expected two handoff snapshots and one run summary, got %d", len(snapshots))
	}
}

func TestValidateRejectsBranchingHandoffChain(t *testing.T) {
	snapshot := executorTestChainSnapshot(t.TempDir(), "/bin/echo", []graph.Edge{
		{ID: "edge_1", From: "planner", To: "implementer", Mode: "handoff"},
		{ID: "edge_2", From: "planner", To: "reviewer", Mode: "handoff"},
	})
	executor := &Executor{}
	_, _, err := executor.Validate(Request{Snapshot: snapshot, AgentID: "planner", Prompt: "task"})
	if err == nil || !strings.Contains(err.Error(), "multiple outgoing graph edges") {
		t.Fatalf("expected branching validation error, got %v", err)
	}
}

func TestValidateRejectsCyclicHandoffChain(t *testing.T) {
	snapshot := executorTestChainSnapshot(t.TempDir(), "/bin/echo", []graph.Edge{
		{ID: "edge_1", From: "planner", To: "implementer", Mode: "handoff"},
		{ID: "edge_2", From: "implementer", To: "planner", Mode: "handoff"},
	})
	executor := &Executor{}
	_, _, err := executor.Validate(Request{Snapshot: snapshot, AgentID: "planner", Prompt: "task"})
	if err == nil || !strings.Contains(err.Error(), "contains a cycle") {
		t.Fatalf("expected cycle validation error, got %v", err)
	}
}

func TestExecuteModelResolvesLocalProfileReference(t *testing.T) {
	db, configPath := newExecutorTestDB(t)
	t.Setenv("NOMICI_TEST_API_KEY", "secret-key")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Fatalf("expected bearer auth from local profile, got %q", got)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "profile resolved"}}],
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`))
	}))
	defer server.Close()
	if err := providers.NewStore(db).Save(context.Background(), &providers.Profile{
		ID:        "local_profile",
		Name:      "Local profile",
		Kind:      providers.KindOpenAICompatible,
		BaseURL:   server.URL,
		Model:     "test-model",
		APIKeyEnv: "NOMICI_TEST_API_KEY",
	}); err != nil {
		t.Fatalf("save provider profile: %v", err)
	}
	snapshot := &graph.Snapshot{
		SnapshotID:    "graph_profile",
		SchemaVersion: "0.1",
		ProjectID:     "test-project",
		CreatedAt:     time.Now().UTC(),
		SourceHash:    "sha256:test",
		IR: graph.IR{
			Models: map[string]graph.Model{
				"primary": {ID: "primary", Profile: "local_profile"},
			},
			Agents: map[string]graph.Agent{
				"assistant": {ID: "assistant", Kind: agentspec.AgentKindModel, Model: "primary"},
			},
		},
	}

	result, err := DBExecutor(db, adapters.NewModelAdapter(), secrets.NewResolver(), configPath).Execute(context.Background(), Request{
		Snapshot: snapshot,
		AgentID:  "assistant",
		Prompt:   "hello",
		RunID:    "run_profile",
	})
	if err != nil {
		t.Fatalf("execute profile-backed model: %v", err)
	}
	if result.Status != adapters.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if len(result.Messages) != 1 || result.Messages[0].Content != "profile resolved" {
		t.Fatalf("unexpected result messages: %+v", result.Messages)
	}
}

func newExecutorTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "nomici.yaml")
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db, configPath
}

func executorTestChainSnapshot(workspace string, script string, edges []graph.Edge) *graph.Snapshot {
	runtime := func(id string, label string) graph.Runtime {
		return graph.Runtime{
			ID:        id,
			Kind:      agentspec.RuntimeKindCLIAgent,
			Workspace: workspace,
			Invoke: graph.RuntimeInvoke{
				Executable: script,
				Args:       []string{"${INPUT}"},
			},
			Env:          map[string]string{"LABEL": label},
			Capabilities: map[string]any{"files_write": false},
		}
	}
	return &graph.Snapshot{
		SnapshotID:    "graph_chain",
		SchemaVersion: "0.1",
		ProjectID:     "test-project",
		CreatedAt:     time.Now().UTC(),
		SourceHash:    "sha256:test",
		IR: graph.IR{
			Models: map[string]graph.Model{},
			Runtimes: map[string]graph.Runtime{
				"planner_runtime":     runtime("planner_runtime", "planner"),
				"implementer_runtime": runtime("implementer_runtime", "implementer"),
				"reviewer_runtime":    runtime("reviewer_runtime", "reviewer"),
			},
			Agents: map[string]graph.Agent{
				"planner":     {ID: "planner", Kind: agentspec.AgentKindExternal, Runtime: "planner_runtime", Role: "Plan work."},
				"implementer": {ID: "implementer", Kind: agentspec.AgentKindExternal, Runtime: "implementer_runtime", Role: "Implement work."},
				"reviewer":    {ID: "reviewer", Kind: agentspec.AgentKindExternal, Runtime: "reviewer_runtime", Role: "Review work."},
			},
			Edges: edges,
		},
	}
}

func writeExecutorTestScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent.sh")
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func assertTraceCount(t *testing.T, events []*tracepkg.Event, eventType string, expected int) {
	t.Helper()
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	if count != expected {
		t.Fatalf("expected %d %s events, got %d", expected, eventType, count)
	}
}
