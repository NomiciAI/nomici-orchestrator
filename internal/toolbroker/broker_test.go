package toolbroker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/NomiciAI/nomici-orchestrator/internal/worklocks"
)

func TestBrokerExecutesWorkspaceReadWithinBoundary(t *testing.T) {
	ctx := context.Background()
	broker, session := testBroker(t)

	record, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		ToolID:    ToolListFiles,
		Input:     map[string]any{"path": "."},
	})
	if err != nil {
		t.Fatalf("execute list files: %v", err)
	}
	if record.Status != StatusCompleted {
		t.Fatalf("expected completed call, got %+v", record)
	}
	if !strings.Contains(record.OutputPreview, "README.md") {
		t.Fatalf("expected listed file, got %q", record.OutputPreview)
	}

	_, err = broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		ToolID:    ToolReadFile,
		Input:     map[string]any{"path": "../outside"},
	})
	if err == nil {
		t.Fatalf("expected boundary error")
	}
}

func TestBrokerRequiresApprovalForWorkspaceMutation(t *testing.T) {
	ctx := context.Background()
	broker, session := testBroker(t)

	pending, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		ToolID:    ToolWriteFile,
		Input:     map[string]any{"path": "notes.txt", "content": "approved"},
	})
	if err != nil {
		t.Fatalf("request write approval: %v", err)
	}
	if pending.Status != StatusWaitingApproval || pending.ApprovalID == "" {
		t.Fatalf("expected waiting approval, got %+v", pending)
	}
	if _, err := broker.Policy.Grant(ctx, pending.ApprovalID, policy.ScopeOnce); err != nil {
		t.Fatalf("grant approval: %v", err)
	}

	completed, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		ToolID:    ToolWriteFile,
		Input:     map[string]any{"path": "notes.txt", "content": "approved"},
	})
	if err != nil {
		t.Fatalf("execute approved write: %v", err)
	}
	if completed.Status != StatusCompleted {
		t.Fatalf("expected completed write, got %+v", completed)
	}
	path := filepath.Join(testWorkspaceRoot(t, broker, session.RunID), "notes.txt")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(payload) != "approved" {
		t.Fatalf("unexpected file content %q", string(payload))
	}
}

func TestBrokerLocksWorkspaceMutation(t *testing.T) {
	ctx := context.Background()
	broker, session := testBroker(t)
	pending, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		TaskID:    "task_write",
		ToolID:    ToolWriteFile,
		Input:     map[string]any{"path": "locked.txt", "content": "locked"},
	})
	if err != nil {
		t.Fatalf("request write approval: %v", err)
	}
	if _, err := broker.Policy.Grant(ctx, pending.ApprovalID, policy.ScopeOnce); err != nil {
		t.Fatalf("grant approval: %v", err)
	}
	completed, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		TaskID:    "task_write",
		ToolID:    ToolWriteFile,
		Input:     map[string]any{"path": "locked.txt", "content": "locked"},
	})
	if err != nil {
		t.Fatalf("execute approved write: %v", err)
	}
	locks, err := broker.Locks.ListByRun(ctx, session.RunID, "", 10)
	if err != nil {
		t.Fatalf("list locks: %v", err)
	}
	if len(locks) != 1 || locks[0].Status != worklocks.StatusReleased || locks[0].ToolCallID != completed.ToolCallID {
		t.Fatalf("expected released lock for tool call, got %+v", locks)
	}
}

func TestBrokerBlocksCriticalBashCommand(t *testing.T) {
	ctx := context.Background()
	broker, session := testBroker(t)
	failed, err := broker.Execute(ctx, ExecuteRequest{
		SessionID: session.SessionID,
		RunID:     session.RunID,
		ToolID:    ToolBash,
		Input:     map[string]any{"command": "rm -rf /", "cwd": "."},
	})
	if err == nil {
		t.Fatalf("expected critical command to fail")
	}
	if failed == nil || failed.Status != StatusFailed || !strings.Contains(failed.Error, "safety policy") {
		t.Fatalf("expected safety failure, got %+v err=%v", failed, err)
	}
}

func TestContainerShellCommandBuildsWorkspaceMount(t *testing.T) {
	workspace := t.TempDir()
	record := &sandbox.Record{
		Mode:          sandbox.ModeContainer,
		WorkspaceRoot: workspace,
		RuntimeBinary: "docker",
		Metadata:      json.RawMessage(`{"container_image":"alpine:3.20"}`),
	}
	cmd, err := containerShellCommand(context.Background(), record, filepath.Join(workspace, "src"), "pwd")
	if err != nil {
		t.Fatalf("build container command: %v", err)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "-v "+workspace+":/workspace") ||
		!strings.Contains(args, "-w /workspace/src") ||
		!strings.Contains(args, "alpine:3.20") {
		t.Fatalf("unexpected container args: %v", cmd.Args)
	}
}

func testBroker(t *testing.T) (*Broker, *runs.Session) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "nomici.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	runStore := runs.NewStore(db)
	session, err := runStore.CreateSession(ctx, runs.CreateSessionRequest{
		RunID:           "run_test",
		ProjectID:       "project_test",
		GraphSnapshotID: "snapshot_test",
		Title:           "test",
		SourceChannel:   "test",
		Status:          runs.SessionStatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandboxStore := sandbox.NewStore(db)
	if _, err := sandboxStore.CreateForRun(ctx, sandbox.CreateRecordRequest{
		RunID:         session.RunID,
		ProjectID:     session.ProjectID,
		Intent:        sandbox.Intent{Mode: sandbox.ModeLocal, BashEnabled: true, FileWriteEnabled: true},
		BaseDir:       workspace,
		WorkspaceRoot: workspace,
		ArtifactRoot:  filepath.Join(workspace, "artifacts"),
	}); err != nil {
		t.Fatal(err)
	}
	return &Broker{
		Store:     NewStore(db),
		Policy:    policy.NewService(db),
		Trace:     trace.NewStore(db),
		Runs:      runStore,
		Sandboxes: sandboxStore,
		Artifacts: artifacts.NewStore(db),
		Locks:     worklocks.NewStore(db),
	}, session
}

func testWorkspaceRoot(t *testing.T, broker *Broker, runID string) string {
	t.Helper()
	record, err := broker.Sandboxes.GetByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return record.WorkspaceRoot
}
