package adapters

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCodexCLIAdapterInvoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-only")
	}
	dir := t.TempDir()
	executable := installCodexAdapterFixture(t, dir)

	result, err := (&CodexCLIAdapter{Executable: executable}).Invoke(context.Background(), "gpt-test", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "Say hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %+v", result)
	}
	if len(result.Messages) != 1 || strings.TrimSpace(result.Messages[0].Content) != "codex fixture response" {
		t.Fatalf("unexpected messages: %+v", result.Messages)
	}
}

func TestCodexCLIAdapterUsesResolvedExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-only")
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	installCodexAdapterFixture(t, binDir)
	codexHome := filepath.Join(dir, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("NOMICI_CODEX_APP_EXECUTABLES", "")

	result, err := NewCodexCLIAdapter().Invoke(context.Background(), "gpt-test", InvokeRequest{
		Messages: []Message{{Role: "user", Content: "Say hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed through resolved executable, got %+v", result)
	}
}

func installCodexAdapterFixture(t *testing.T, dir string) string {
	t.Helper()
	executable := filepath.Join(dir, "codex")
	script := `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    shift
    out="$1"
  fi
  shift
done
cat >/dev/null
printf "codex fixture response" > "$out"
`
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return executable
}
