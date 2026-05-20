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
	bySession, err := runStore.GetBySession(ctx, session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if bySession.Session.RunID != "run_test" || len(bySession.Tasks) != 1 {
		t.Fatalf("expected session detail by session id, got %+v", bySession)
	}
	tasks, err := runStore.ListTasksBySession(ctx, session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != task.TaskID {
		t.Fatalf("expected task list by session id, got %+v", tasks)
	}
}

func TestStoreCancelsRunningSessionAndTasks(t *testing.T) {
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
		RunID:           "run_cancel",
		ProjectID:       "project",
		GraphSnapshotID: "graph",
		Title:           "Cancel task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStore.CreateTask(ctx, CreateTaskRequest{RunID: "run_cancel", AgentID: "planner", Status: TaskStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if _, err := runStore.CreateTask(ctx, CreateTaskRequest{RunID: "run_cancel", AgentID: "reporter", Status: TaskStatusQueued}); err != nil {
		t.Fatal(err)
	}

	if err := runStore.CancelSession(ctx, session.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := runStore.CancelTasks(ctx, "run_cancel"); err != nil {
		t.Fatal(err)
	}
	detail, err := runStore.GetBySession(ctx, session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Session.Status != SessionStatusCancelled {
		t.Fatalf("expected cancelled session, got %+v", detail.Session)
	}
	for _, task := range detail.Tasks {
		if task.Status != TaskStatusCancelled {
			t.Fatalf("expected cancelled task, got %+v", detail.Tasks)
		}
	}
	if err := runStore.CancelSession(ctx, session.SessionID); err == nil {
		t.Fatal("expected cancelling a terminal session to fail")
	}
}

func TestStoreUpdatesTaskMetadataAndResumesPlanReview(t *testing.T) {
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
		RunID:           "run_review",
		ProjectID:       "project",
		GraphSnapshotID: "graph",
		Title:           "Review plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := runStore.CreateTask(ctx, CreateTaskRequest{RunID: "run_review", AgentID: "planner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runStore.UpdateTaskStatus(ctx, task.TaskID, TaskStatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := runStore.UpdateTaskMetadata(ctx, task.TaskID, []byte(`{"summary":"planned"}`)); err != nil {
		t.Fatal(err)
	}
	if err := runStore.AddTaskArtifactRef(ctx, task.TaskID, "artifact_1"); err != nil {
		t.Fatal(err)
	}
	if err := runStore.UpdateSessionStatus(ctx, "run_review", SessionStatusPlanReview); err != nil {
		t.Fatal(err)
	}
	if err := runStore.ResumeSession(ctx, session.SessionID); err != nil {
		t.Fatal(err)
	}
	detail, err := runStore.GetBySession(ctx, session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Session.Status != SessionStatusRunning {
		t.Fatalf("expected resumed session, got %+v", detail.Session)
	}
	if string(detail.Tasks[0].Metadata) != `{"summary":"planned"}` || len(detail.Tasks[0].ArtifactRefs) != 1 {
		t.Fatalf("expected task metadata and artifact refs, got %+v", detail.Tasks[0])
	}
}
