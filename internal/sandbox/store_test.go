package sandbox

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestStoreCreatesAndReleasesSandboxRecord(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	runID := "run_sandbox"
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	record, err := NewStore(db).CreateForRun(context.Background(), CreateRecordRequest{
		RunID:         runID,
		TaskID:        "task_1",
		ProjectID:     "project",
		Intent:        Intent{Mode: ModeLocal, BashEnabled: true, FileWriteEnabled: true},
		WorkspaceRoot: workspaceRoot,
		ArtifactRoot:  artifactRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.SandboxID != DeterministicID(runID) {
		t.Fatalf("expected deterministic sandbox id, got %q", record.SandboxID)
	}
	if record.Provider != ProviderLocalWorkspace || record.Status != StatusAvailable {
		t.Fatalf("unexpected sandbox provider/status: %+v", record)
	}

	loaded, err := NewStore(db).GetByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.WorkspaceRoot != workspaceRoot || loaded.ArtifactRoot != artifactRoot {
		t.Fatalf("unexpected roots: %+v", loaded)
	}

	if err := NewStore(db).ReleaseByRun(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	released, err := NewStore(db).GetByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if released.CleanupStatus != CleanupReleased || released.ReleasedAt == nil {
		t.Fatalf("expected released sandbox, got %+v", released)
	}
}

func TestIntentFromDeploymentDefaultsInvalidShapeToLocal(t *testing.T) {
	intent := IntentFromDeployment(map[string]any{"sandbox": "container"})
	if intent.Mode != ModeLocal {
		t.Fatalf("expected local default, got %+v", intent)
	}

	intent = IntentFromDeployment(map[string]any{"sandbox": map[string]any{
		"mode":               ModeContainer,
		"bash_enabled":       true,
		"file_write_enabled": true,
	}})
	if intent.Mode != ModeContainer || !intent.BashEnabled || !intent.FileWriteEnabled {
		t.Fatalf("unexpected intent: %+v", intent)
	}
}
