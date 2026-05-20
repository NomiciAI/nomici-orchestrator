package artifacts

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestArtifactRevisionsTrackCreateAndRevise(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	artifactStore := NewStore(db)
	artifact, err := artifactStore.Create(ctx, CreateRequest{
		SessionID: "session",
		RunID:     "run",
		Type:      TypePlan,
		Title:     "Plan",
		Preview:   "first plan",
	})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if _, err := artifactStore.Revise(ctx, artifact.ArtifactID, "second plan"); err != nil {
		t.Fatalf("revise artifact: %v", err)
	}
	revisions, err := artifactStore.ListRevisions(ctx, artifact.ArtifactID, 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected two revisions, got %d", len(revisions))
	}
	if revisions[0].Revision != 2 || revisions[0].DiffPreview == "" {
		t.Fatalf("expected latest revision with diff, got %+v", revisions[0])
	}
}
