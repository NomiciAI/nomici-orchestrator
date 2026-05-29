package cli

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
)

func TestAuthenticatedConsoleURLUsesBootstrapFragment(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	openURL, err := authenticatedConsoleURL("http://127.0.0.1:8787", dbPath)
	if err != nil {
		t.Fatalf("create authenticated console URL: %v", err)
	}
	parsed, err := url.Parse(openURL)
	if err != nil {
		t.Fatalf("parse authenticated console URL: %v", err)
	}
	token := parsed.Query().Get("bootstrap_token")
	if token != "" {
		t.Fatal("bootstrap token must not be sent as a query parameter")
	}
	token = parsed.Fragment
	values, err := url.ParseQuery(token)
	if err != nil {
		t.Fatalf("parse bootstrap fragment: %v", err)
	}
	token = values.Get("bootstrap_token")
	if token == "" {
		t.Fatalf("expected bootstrap token in fragment, got %q", parsed.Fragment)
	}
	ok, err := gatewayauth.ConsumeBootstrap(
		gatewayauth.BootstrapPathForDB(dbPath),
		token,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("consume bootstrap token: %v", err)
	}
	if !ok {
		t.Fatal("expected bootstrap token to be valid")
	}
}
