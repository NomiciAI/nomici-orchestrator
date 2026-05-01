package gatewayauth

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreateCreatesStrictTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.token")

	token, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load or create token: %v", err)
	}
	if !created {
		t.Fatal("expected token to be created")
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	loaded, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load existing token: %v", err)
	}
	if created {
		t.Fatal("expected existing token to be reused")
	}
	if loaded != token {
		t.Fatalf("expected reused token %q, got %q", token, loaded)
	}
}

func TestMatches(t *testing.T) {
	if !Matches("token", "token") {
		t.Fatal("expected token to match")
	}
	if Matches("token", "other") {
		t.Fatal("expected token mismatch")
	}
	if Matches("", "") {
		t.Fatal("empty token must not match")
	}
}
