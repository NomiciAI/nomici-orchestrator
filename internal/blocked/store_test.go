package blocked

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestBlockedActionLifecycle(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	blockedStore := NewStore(db)
	action, err := blockedStore.Create(context.Background(), CreateRequest{
		SessionID:      "session_test",
		RunID:          "run_test",
		TaskID:         "task_test",
		Kind:           KindToolApproval,
		Title:          "Approve tool call",
		RequiredAction: "grant_or_deny_approval",
		ApprovalID:     "approval_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != StatusOpen {
		t.Fatalf("expected open action, got %+v", action)
	}
	actions, err := blockedStore.ListBySession(context.Background(), "session_test", StatusOpen, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].ApprovalID != "approval_test" {
		t.Fatalf("expected listed action, got %+v", actions)
	}
	queue, err := blockedStore.List(context.Background(), StatusOpen, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].BlockedActionID != action.BlockedActionID {
		t.Fatalf("expected review queue action, got %+v", queue)
	}
	if err := blockedStore.ResolveByApproval(context.Background(), "approval_test", StatusResolved); err != nil {
		t.Fatal(err)
	}
	resolved, err := blockedStore.Get(context.Background(), action.BlockedActionID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != StatusResolved || resolved.ResolvedAt == "" {
		t.Fatalf("expected resolved action, got %+v", resolved)
	}
}
