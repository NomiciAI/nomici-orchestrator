package agentspec

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSpec(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nomici.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}

func TestValidateMinimalGatewayAgent(t *testing.T) {
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
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if errors := Validate(loaded); len(errors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errors)
	}
}

func TestValidateRejectsNativeAgent(t *testing.T) {
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
    kind: native_agent
    model: gpt
`)
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	errors := Validate(loaded)
	if len(errors) != 1 {
		t.Fatalf("expected one validation error, got %+v", errors)
	}
	if errors[0].Code != "unsupported_agent_kind" {
		t.Fatalf("expected unsupported_agent_kind, got %s", errors[0].Code)
	}
	if errors[0].Source.Path != "agents.product_pm.kind" {
		t.Fatalf("expected source path agents.product_pm.kind, got %s", errors[0].Source.Path)
	}
	if errors[0].Source.Line == 0 {
		t.Fatal("expected source line")
	}
}

func TestValidateMissingModelReference(t *testing.T) {
	path := writeSpec(t, `
version: "0.1"
project:
  name: demo
agents:
  product_pm:
    kind: gateway_agent
    model: missing
`)
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	errors := Validate(loaded)
	if len(errors) != 1 {
		t.Fatalf("expected one validation error, got %+v", errors)
	}
	if errors[0].Code != "missing_reference" {
		t.Fatalf("expected missing_reference, got %s", errors[0].Code)
	}
}

func TestValidateExternalAgentCLIRuntime(t *testing.T) {
	path := writeSpec(t, `
version: "0.1"
project:
  name: demo
runtimes:
  implementer_cli:
    kind: cli_agent
    runner: cli_invoke
    workspace: ./workspace
    invoke:
      executable: fake-agent
      args:
        - "${INPUT}"
agents:
  implementer:
    kind: external_agent
    runtime: implementer_cli
`)
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if errors := Validate(loaded); len(errors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", errors)
	}
}

func TestValidateCLIRuntimeRequiresExecutable(t *testing.T) {
	path := writeSpec(t, `
version: "0.1"
project:
  name: demo
runtimes:
  implementer_cli:
    kind: cli_agent
agents:
  implementer:
    kind: external_agent
    runtime: implementer_cli
`)
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	errors := Validate(loaded)
	if len(errors) != 1 {
		t.Fatalf("expected one validation error, got %+v", errors)
	}
	if errors[0].Code != "missing_required" {
		t.Fatalf("expected missing_required, got %s", errors[0].Code)
	}
	if errors[0].Source.Path != "runtimes.implementer_cli.invoke.executable" {
		t.Fatalf("expected runtime executable source, got %s", errors[0].Source.Path)
	}
}
