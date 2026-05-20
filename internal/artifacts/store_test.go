package artifacts

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestStoreCreatesRevisesAndListsArtifacts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	artifactStore := NewStore(db)
	ctx := context.Background()

	artifact, err := artifactStore.Create(ctx, CreateRequest{
		SessionID: "session_1",
		RunID:     "run_1",
		TaskID:    "task_1",
		Type:      TypePlan,
		Title:     "Plan",
		Preview:   "Initial plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ArtifactID == "" || artifact.Revision != 1 || artifact.ReviewState != ReviewDraft {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	revised, err := artifactStore.Revise(ctx, artifact.ArtifactID, "Revised plan")
	if err != nil {
		t.Fatal(err)
	}
	if revised.Revision != 2 || revised.ReviewState != ReviewRevised || revised.Preview != "Revised plan" {
		t.Fatalf("unexpected revised artifact: %+v", revised)
	}
	approved, err := artifactStore.SetReviewState(ctx, artifact.ArtifactID, ReviewApproved)
	if err != nil {
		t.Fatal(err)
	}
	if approved.ReviewState != ReviewApproved {
		t.Fatalf("expected approved artifact, got %+v", approved)
	}
	artifacts, err := artifactStore.List(ctx, "session_1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].ArtifactID != artifact.ArtifactID {
		t.Fatalf("expected listed artifact, got %+v", artifacts)
	}
}
