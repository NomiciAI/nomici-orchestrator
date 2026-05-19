package runs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestStoreCreatesSessionAndTasks(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	runStore := NewStore(db)
	ctx := context.Background()

	session, err := runStore.CreateSession(ctx, CreateSessionRequest{
		RunID:           "run_test",
		ProjectID:       "project",
		GraphSnapshotID: "graph",
		Title:           "Ship task ledger",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.SessionID == "" || session.Status != SessionStatusRunning {
		t.Fatalf("unexpected session: %+v", session)
	}

	task, err := runStore.CreateTask(ctx, CreateTaskRequest{
		RunID:   "run_test",
		AgentID: "planner",
		Status:  TaskStatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.TaskID == "" || task.Status != TaskStatusRunning {
		t.Fatalf("unexpected task: %+v", task)
	}

	if err := runStore.CompleteTasks(ctx, "run_test", TaskStatusCompleted); err != nil {
		t.Fatal(err)
	}
	if err := runStore.CompleteSession(ctx, "run_test", SessionStatusCompleted); err != nil {
		t.Fatal(err)
	}

	detail, err := runStore.GetByRun(ctx, "run_test")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Session.Status != SessionStatusCompleted {
		t.Fatalf("expected completed session, got %+v", detail.Session)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].Status != TaskStatusCompleted {
		t.Fatalf("expected completed task, got %+v", detail.Tasks)
	}
}
