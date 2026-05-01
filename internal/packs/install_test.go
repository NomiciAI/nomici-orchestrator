package packs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestInstallDeveloperTeamCreatesRunnableSpec(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	configPath := filepath.Join(dir, "nomici.yaml")
	saveTestProvider(t, dbPath)

	result, err := InstallDeveloperTeam(context.Background(), InstallOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
	})
	if err != nil {
		t.Fatalf("install pack: %v", err)
	}
	if !result.Created {
		t.Fatal("expected config to be created")
	}
	if result.ModelID != "gpt" {
		t.Fatalf("expected model gpt, got %s", result.ModelID)
	}

	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load installed spec: %v", err)
	}
	if errors := agentspec.Validate(loaded); len(errors) != 0 {
		t.Fatalf("expected installed spec to validate, got %+v", errors)
	}
	if loaded.Spec.Agents["product_pm"].Kind != agentspec.AgentKindGateway {
		t.Fatalf("expected product_pm gateway agent")
	}
	if loaded.Spec.Agents["architect"].Kind != agentspec.AgentKindModel {
		t.Fatalf("expected architect model agent")
	}
}

func TestInstallDeveloperTeamRequiresProvider(t *testing.T) {
	dir := t.TempDir()
	_, err := InstallDeveloperTeam(context.Background(), InstallOptions{
		ConfigPath: filepath.Join(dir, "nomici.yaml"),
		DBPath:     filepath.Join(dir, "state.db"),
	})
	if err == nil {
		t.Fatal("expected missing provider error")
	}
}

func saveTestProvider(t *testing.T, dbPath string) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := providers.NewStore(db).Save(context.Background(), &providers.Profile{
		ID:        "gpt",
		Name:      "gpt",
		Kind:      providers.KindOpenAICompatible,
		BaseURL:   "http://127.0.0.1:19999/v1",
		Model:     "fake-model",
		APIKeyEnv: "OPENAI_API_KEY",
		Capabilities: map[string]string{
			"streaming": "true",
		},
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
}
