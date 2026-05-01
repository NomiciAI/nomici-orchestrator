package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestCompileMinimalGraph(t *testing.T) {
	path := writeSpec(t, `
version: "0.1"
project:
  name: demo
models:
  gpt:
    kind: openai_compatible
    base_url: http://127.0.0.1:18999/v1
    api_key_env: OPENAI_API_KEY
    model: fake-model
agents:
  product_pm:
    kind: gateway_agent
    model: gpt
    role: Coordinate.
`)
	loaded, err := agentspec.LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	snapshot, errors := Compile(loaded)
	if len(errors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errors)
	}
	if snapshot.ProjectID != "demo" {
		t.Fatalf("expected project demo, got %s", snapshot.ProjectID)
	}
	if snapshot.IR.Agents["product_pm"].Model != "gpt" {
		t.Fatalf("expected agent model gpt")
	}
	if snapshot.IR.Models["gpt"].Source.Path != "models.gpt" {
		t.Fatalf("expected model source path, got %s", snapshot.IR.Models["gpt"].Source.Path)
	}
}

func TestGraphStoreSaveLatest(t *testing.T) {
	path := writeSpec(t, `
version: "0.1"
project:
  name: demo
models:
  gpt:
    kind: openai_compatible
    base_url: http://127.0.0.1:18999/v1
    api_key_env: OPENAI_API_KEY
    model: fake-model
agents:
  product_pm:
    kind: gateway_agent
    model: gpt
`)
	loaded, err := agentspec.LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	snapshot, errors := Compile(loaded)
	if len(errors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errors)
	}

	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	graphStore := NewStore(db)
	if err := graphStore.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	latest, err := graphStore.Latest(context.Background())
	if err != nil {
		t.Fatalf("load latest snapshot: %v", err)
	}
	if latest.SnapshotID != snapshot.SnapshotID {
		t.Fatalf("expected snapshot %s, got %s", snapshot.SnapshotID, latest.SnapshotID)
	}
}

func writeSpec(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nomici.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}
