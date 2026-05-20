package memory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestMemoryProposalApprovalPromotesSharedContext(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "nomici.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	memoryStore := NewStore(db)
	proposal, err := memoryStore.Create(ctx, CreateRequest{
		ProjectID:  "project",
		SessionID:  "session",
		RunID:      "run",
		SourceType: "session_summary",
		Title:      "Reusable context",
		Body:       "Use mediated tools for workspace changes.",
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	approved, err := memoryStore.Approve(ctx, proposal.ProposalID, sharedcontext.NewStore(db))
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	if approved.Status != StatusApproved || approved.ContextID == "" {
		t.Fatalf("expected approved proposal with context id, got %+v", approved)
	}
}
