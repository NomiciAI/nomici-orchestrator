package policy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestCheckAllowsReadOnlyCLIAgent(t *testing.T) {
	service := newTestService(t)
	decision, err := service.Check(context.Background(), ActionRequest{
		RunID:      "run_1",
		ActionType: ActionCLIInvoke,
		ProjectID:  "demo",
		Workspace:  t.TempDir(),
		FilesWrite: false,
	})
	if err != nil {
		t.Fatalf("check policy: %v", err)
	}
	if decision.Decision != DecisionAllow || decision.Risk != RiskLow {
		t.Fatalf("expected low allow, got %+v", decision)
	}
}

func TestCheckRequestsApprovalForMutableCLIAgent(t *testing.T) {
	service := newTestService(t)
	decision, err := service.Check(context.Background(), ActionRequest{
		RunID:      "run_1",
		ActionType: ActionCLIInvoke,
		ProjectID:  "demo",
		AgentID:    "implementer",
		RuntimeID:  "implementer_cli",
		Workspace:  t.TempDir(),
		FilesWrite: true,
	})
	if err != nil {
		t.Fatalf("check policy: %v", err)
	}
	if decision.Decision != DecisionApproval || decision.Risk != RiskMedium {
		t.Fatalf("expected medium approval, got %+v", decision)
	}
	if decision.ApprovalID == "" {
		t.Fatal("expected approval id")
	}
}

func TestGrantOnceIsConsumedByNextMatchingAction(t *testing.T) {
	service := newTestService(t)
	workspace := t.TempDir()
	action := ActionRequest{
		RunID:      "run_1",
		ActionType: ActionCLIInvoke,
		ProjectID:  "demo",
		Workspace:  workspace,
		FilesWrite: true,
	}
	decision, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check policy: %v", err)
	}
	if _, err := service.Grant(context.Background(), decision.ApprovalID, ScopeOnce); err != nil {
		t.Fatalf("grant approval: %v", err)
	}

	action.RunID = "run_2"
	allowed, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check granted policy: %v", err)
	}
	if allowed.Decision != DecisionAllow || allowed.Scope != ScopeOnce {
		t.Fatalf("expected once allow, got %+v", allowed)
	}
	pending, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check consumed policy: %v", err)
	}
	if pending.Decision != DecisionApproval {
		t.Fatalf("expected once approval to be consumed, got %+v", pending)
	}
}

func TestGrantRunBindsToCurrentRun(t *testing.T) {
	service := newTestService(t)
	workspace := t.TempDir()
	action := ActionRequest{
		RunID:      "run_1",
		ActionType: ActionCLIInvoke,
		ProjectID:  "demo",
		Workspace:  workspace,
		FilesWrite: true,
	}
	decision, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check policy: %v", err)
	}
	if _, err := service.Grant(context.Background(), decision.ApprovalID, ScopeRun); err != nil {
		t.Fatalf("grant approval: %v", err)
	}

	action.RunID = "run_2"
	allowed, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check run policy: %v", err)
	}
	if allowed.Decision != DecisionAllow || allowed.Scope != ScopeRun {
		t.Fatalf("expected run allow, got %+v", allowed)
	}
	allowedAgain, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check run policy again: %v", err)
	}
	if allowedAgain.Decision != DecisionAllow || allowedAgain.Scope != ScopeRun {
		t.Fatalf("expected repeated run allow, got %+v", allowedAgain)
	}
	action.RunID = "run_3"
	pending, err := service.Check(context.Background(), action)
	if err != nil {
		t.Fatalf("check new run policy: %v", err)
	}
	if pending.Decision != DecisionApproval {
		t.Fatalf("expected new run to require approval, got %+v", pending)
	}
}

func TestCriticalWorkspaceDenied(t *testing.T) {
	service := newTestService(t)
	decision, err := service.Check(context.Background(), ActionRequest{
		RunID:      "run_1",
		ActionType: ActionCLIInvoke,
		ProjectID:  "demo",
		Workspace:  filepath.Join(string(filepath.Separator), "etc"),
		FilesWrite: true,
	})
	if err != nil {
		t.Fatalf("check policy: %v", err)
	}
	if decision.Decision != DecisionDeny || decision.Risk != RiskCritical {
		t.Fatalf("expected critical deny, got %+v", decision)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return NewService(db)
}
