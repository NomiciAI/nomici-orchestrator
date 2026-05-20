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
