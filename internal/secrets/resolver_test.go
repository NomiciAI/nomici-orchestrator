package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEnv(t *testing.T) {
	t.Setenv("NOMICI_TEST_SECRET", "secret-value")

	resolver := NewResolver()
	value, ok := resolver.ResolveEnv("NOMICI_TEST_SECRET")
	if !ok {
		t.Fatal("expected env var to resolve")
	}
	if value != "secret-value" {
		t.Fatalf("expected secret-value, got %q", value)
	}
}

func TestResolveEnvMissing(t *testing.T) {
	resolver := NewResolver()
	if value, ok := resolver.ResolveEnv("NOMICI_MISSING_SECRET"); ok || value != "" {
		t.Fatalf("expected missing env var, got %q", value)
	}
}

func TestResolveLocalSecretFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(".nomici", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".nomici", "secrets.env"), []byte("NOMICI_LOCAL_SECRET=local-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolver()
	value, ok := resolver.ResolveEnv("NOMICI_LOCAL_SECRET")
	if !ok {
		t.Fatal("expected local secret file to resolve")
	}
	if value != "local-value" {
		t.Fatalf("expected local-value, got %q", value)
	}
}

func TestResolveLocalSecretFileFromConfigDir(t *testing.T) {
	dir := t.TempDir()
	secretDir := filepath.Join(dir, ".nomici")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "secrets.env"), []byte("NOMICI_CONFIG_SECRET=config-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolver := NewResolverForConfig(filepath.Join(dir, "nomici.yaml"))
	value, ok := resolver.ResolveEnv("NOMICI_CONFIG_SECRET")
	if !ok {
		t.Fatal("expected config-local secret file to resolve")
	}
	if value != "config-value" {
		t.Fatalf("expected config-value, got %q", value)
	}
}

func TestRedact(t *testing.T) {
	resolver := NewResolver()
	if got := resolver.Redact("sk-test-value"); got != "[redacted]" {
		t.Fatalf("expected redacted placeholder, got %q", got)
	}
	if got := resolver.Redact("localhost"); got != "localhost" {
		t.Fatalf("expected non-sensitive value to pass through, got %q", got)
	}
}

func TestRedactedEnv(t *testing.T) {
	if got := RedactedEnv("OPENAI_API_KEY"); got != "[redacted:OPENAI_API_KEY]" {
		t.Fatalf("expected env redaction placeholder, got %q", got)
	}
}

func TestLooksSensitive(t *testing.T) {
	if !LooksSensitive("sk-short") {
		t.Fatal("expected sk-prefixed value to look sensitive")
	}
	if !LooksSensitive("abcdefghijklmnopqrstuvwxyz") {
		t.Fatal("expected long value to look sensitive")
	}
	if LooksSensitive("localhost") {
		t.Fatal("did not expect localhost to look sensitive")
	}
}
