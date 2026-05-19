package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

func TestSetupCreatesUsableConfigAndSandbox(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/nomici.yaml"
	dbPath := dir + "/state.db"

	output, err := executeRootForTest(
		"setup",
		"--config", configPath,
		"--db-path", dbPath,
		"--provider", "ollama",
		"--name", "local llama",
		"--model", "llama3.2",
		"--pack", "developer-team",
		"--sandbox", "container",
		"--enable-bash",
		"--enable-file-write",
		"--yes",
	)
	if err != nil {
		t.Fatalf("setup failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Setup complete.") {
		t.Fatalf("expected setup summary, got:\n%s", output)
	}

	loaded, exists, err := loadSpecIfExists(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("expected config to exist")
	}
	if _, ok := loaded.Spec.Agents["product_pm"]; !ok {
		t.Fatalf("expected product_pm agent")
	}
	sandbox, ok := loaded.Spec.Deployment["sandbox"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment.sandbox map, got %#v", loaded.Spec.Deployment["sandbox"])
	}
	if sandbox["mode"] != sandboxModeContainer {
		t.Fatalf("expected container sandbox, got %#v", sandbox["mode"])
	}
	if sandbox["bash_enabled"] != true {
		t.Fatalf("expected bash_enabled true, got %#v", sandbox["bash_enabled"])
	}

	db, err := openMigratedDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	profile, err := providers.NewStore(db).Get(t.Context(), "local_llama")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Kind != providers.KindOllama || profile.Model != "llama3.2" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestSetupYesRequiresModel(t *testing.T) {
	dir := t.TempDir()
	output, err := executeRootForTest(
		"setup",
		"--config", dir+"/nomici.yaml",
		"--db-path", dir+"/state.db",
		"--provider", "ollama",
		"--yes",
	)
	if err == nil {
		t.Fatalf("expected setup to require model, got output:\n%s", output)
	}
	if !strings.Contains(err.Error(), "--model is required") {
		t.Fatalf("expected model error, got %v", err)
	}
}

func executeRootForTest(args ...string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewRootCommand("test")
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String() + stderr.String(), err
}
