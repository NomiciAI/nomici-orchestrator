package providers

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return NewStore(db)
}

func TestSaveGetListDeleteProfile(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	profile := &Profile{
		ID:        "gpt",
		Name:      "gpt",
		Kind:      KindOpenAICompatible,
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-5.5",
		APIKeyEnv: "OPENAI_API_KEY",
		Capabilities: map[string]string{
			"tool_calling": "unknown",
		},
	}
	if err := store.Save(ctx, profile); err != nil {
		t.Fatalf("save profile: %v", err)
	}

	got, err := store.Get(ctx, "gpt")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if got.APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("expected API key env reference, got %q", got.APIKeyEnv)
	}
	if got.Capabilities["tool_calling"] != "unknown" {
		t.Fatalf("expected capability to round trip")
	}

	profile.Model = "gpt-5.5-mini"
	if err := store.Save(ctx, profile); err != nil {
		t.Fatalf("update profile: %v", err)
	}
	got, err = store.Get(ctx, "gpt")
	if err != nil {
		t.Fatalf("get updated profile: %v", err)
	}
	if got.Model != "gpt-5.5-mini" {
		t.Fatalf("expected updated model, got %q", got.Model)
	}

	profiles, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected one profile, got %d", len(profiles))
	}

	if err := store.Delete(ctx, "gpt"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	if _, err := store.Get(ctx, "gpt"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no rows after delete, got %v", err)
	}
}

func TestValidateRejectsUnknownKind(t *testing.T) {
	profile := &Profile{
		ID:      "bad",
		Name:    "bad",
		Kind:    "bad",
		BaseURL: "http://localhost",
		Model:   "model",
	}
	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRequiresID(t *testing.T) {
	profile := &Profile{
		Name:      "gpt",
		Kind:      KindOpenAICompatible,
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-5.5",
		APIKeyEnv: "OPENAI_API_KEY",
	}
	if err := profile.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
