package sharedcontext

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestSaveSnapshotRedactsSensitiveText(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	contextStore := NewStore(db)
	token := "sk-" + "123456789abcdef"
	if err := contextStore.SaveSnapshot(context.Background(), &Snapshot{
		ProjectID:  "demo",
		RunID:      "run_1",
		FromAgent:  "implementer",
		ToAgent:    "reviewer",
		Summary:    "used " + token + " and completed work",
		OpenIssues: []string{"check bearer abcdefghijklmnopqrstuv"},
		CreatedBy:  CreatedBy{Kind: "adapter_result", AgentID: "implementer"},
	}); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snapshots, err := contextStore.ListSnapshots(context.Background(), "demo", 10)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if strings.Contains(snapshots[0].Summary, "sk-123") {
		t.Fatalf("expected summary to be redacted, got %q", snapshots[0].Summary)
	}
	if strings.Contains(snapshots[0].OpenIssues[0], "bearer abc") {
		t.Fatalf("expected open issue to be redacted, got %q", snapshots[0].OpenIssues[0])
	}
}

func TestRenderBriefing(t *testing.T) {
	briefing := RenderBriefing(&Snapshot{
		SnapshotID:      "ctxsnap_1",
		Summary:         "Implemented auth middleware.",
		OpenIssues:      []string{"Token rotation deferred."},
		Recommendations: []string{"Review tests."},
		ArtifactRefs:    []string{"diff.txt"},
	})
	if briefing.SnapshotID != "ctxsnap_1" {
		t.Fatalf("expected snapshot id, got %q", briefing.SnapshotID)
	}
	for _, expected := range []string{"Task briefing:", "Implemented auth middleware.", "Token rotation deferred.", "diff.txt"} {
		if !strings.Contains(briefing.Text, expected) {
			t.Fatalf("expected briefing to contain %q, got %q", expected, briefing.Text)
		}
	}
}
