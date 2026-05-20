package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestSessionCommandsListShowTasksAndCancel(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	runStore := runpkg.NewStore(db)
	session, err := runStore.CreateSession(context.Background(), runpkg.CreateSessionRequest{
		RunID:           "run_cli_session",
		ProjectID:       "project",
		GraphSnapshotID: "graph",
		Title:           "CLI session task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStore.CreateTask(context.Background(), runpkg.CreateTaskRequest{
		RunID:     "run_cli_session",
		AgentID:   "planner",
		RuntimeID: "planner_cli",
		Status:    runpkg.TaskStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	output, err := executeRootForTest("session", "--db-path", dbPath, "list")
	if err != nil {
		t.Fatalf("session list failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, session.SessionID) || !strings.Contains(output, "CLI session task") {
		t.Fatalf("expected session list output, got:\n%s", output)
	}

	output, err = executeRootForTest("session", "--db-path", dbPath, "show", session.SessionID)
	if err != nil {
		t.Fatalf("session show failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Tasks:      1") || !strings.Contains(output, "Status:     running") {
		t.Fatalf("expected session show output, got:\n%s", output)
	}

	output, err = executeRootForTest("session", "--db-path", dbPath, "tasks", session.SessionID)
	if err != nil {
		t.Fatalf("session tasks failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "planner") || !strings.Contains(output, "planner_cli") {
		t.Fatalf("expected session task output, got:\n%s", output)
	}

	output, err = executeRootForTest("session", "--db-path", dbPath, "cancel", session.SessionID)
	if err != nil {
		t.Fatalf("session cancel failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Cancelled session "+session.SessionID) {
		t.Fatalf("expected cancel output, got:\n%s", output)
	}
	output, err = executeRootForTest("session", "--db-path", dbPath, "show", session.SessionID)
	if err != nil {
		t.Fatalf("session show after cancel failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Status:     cancelled") {
		t.Fatalf("expected cancelled status, got:\n%s", output)
	}
}
