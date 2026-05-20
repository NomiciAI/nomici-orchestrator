package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentAndOrchestrateCommands(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nomici.yaml")
	dbPath := filepath.Join(dir, "state.db")
	config := `version: "0.1"
project:
  name: test
models:
  default:
    kind: openai_compatible
    base_url: http://127.0.0.1:1
    model: test
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := executeRootForTest(
		"agent", "--config", configPath, "--db-path", dbPath,
		"create", "planner", "--kind", "model_agent", "--model", "default", "--role", "Plan work",
	)
	if err != nil {
		t.Fatalf("agent create failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Agent saved: planner") {
		t.Fatalf("expected save output, got:\n%s", output)
	}

	output, err = executeRootForTest("agent", "--config", configPath, "list")
	if err != nil {
		t.Fatalf("agent list failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "planner") || !strings.Contains(output, "Plan work") {
		t.Fatalf("expected planner in list, got:\n%s", output)
	}

	output, err = executeRootForTest("agent", "--config", configPath, "update", "planner", "--instructions", "Keep outputs short")
	if err != nil {
		t.Fatalf("agent update failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Agent updated: planner") {
		t.Fatalf("expected update output, got:\n%s", output)
	}

	output, err = executeRootForTest("orchestrate", "--config", configPath, "--db-path", dbPath, "set-entrypoint", "planner")
	if err != nil {
		t.Fatalf("orchestrate set-entrypoint failed: %v\n%s", err, output)
	}
	output, err = executeRootForTest("orchestrate", "--config", configPath, "--db-path", dbPath, "role", "reorder", "planner")
	if err != nil {
		t.Fatalf("orchestrate reorder failed: %v\n%s", err, output)
	}
	output, err = executeRootForTest("orchestrate", "--config", configPath, "show")
	if err != nil {
		t.Fatalf("orchestrate show failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Entrypoint: planner") || !strings.Contains(output, "Role order: planner") {
		t.Fatalf("expected orchestration output, got:\n%s", output)
	}
}
