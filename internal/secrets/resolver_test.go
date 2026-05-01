package secrets

import "testing"

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
