package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
	searchTool, ok := loaded.Spec.Tools["web_search"]
	if !ok {
		t.Fatalf("expected web_search tool config")
	}
	if searchTool["provider"] != "duckduckgo" {
		t.Fatalf("expected duckduckgo search provider, got %#v", searchTool["provider"])
	}
	fetchTool, ok := loaded.Spec.Tools["web_fetch"]
	if !ok {
		t.Fatalf("expected web_fetch tool config")
	}
	if fetchTool["provider"] != "jina_reader" {
		t.Fatalf("expected jina_reader fetch provider, got %#v", fetchTool["provider"])
	}
	if !strings.Contains(output, "nomici dev --config") {
		t.Fatalf("expected setup next steps to prefer nomici dev, got:\n%s", output)
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

	doctorOutput, err := executeRootForTest(
		"doctor",
		"--config", configPath,
		"--db-path", dbPath,
		"--gateway-url", "http://127.0.0.1:1",
	)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, doctorOutput)
	}
	if !strings.Contains(doctorOutput, "web_tools") || !strings.Contains(doctorOutput, "search=duckduckgo fetch=jina_reader") {
		t.Fatalf("expected doctor web tool readiness, got:\n%s", doctorOutput)
	}
}

func TestSetupCreatesCodexCLIProviderWhenLocalAuthIsReady(t *testing.T) {
	dir := t.TempDir()
	installFakeCodexCLI(t)
	codexHome := filepath.Join(dir, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", codexHome)

	output, err := executeRootForTest(
		"setup",
		"--config", filepath.Join(dir, "nomici.yaml"),
		"--db-path", filepath.Join(dir, "state.db"),
		"--provider", "codex-cli",
		"--name", "codex local",
		"--model", "gpt-5.4",
		"--pack", "none",
		"--sandbox", "local",
		"--yes",
	)
	if err != nil {
		t.Fatalf("setup failed: %v\n%s", err, output)
	}

	db, err := openMigratedDB(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	profile, err := providers.NewStore(db).Get(t.Context(), "codex_local")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Kind != providers.KindCodexCLI {
		t.Fatalf("expected codex_cli profile, got %+v", profile)
	}
	if profile.APIKeyEnv != "" {
		t.Fatalf("expected Codex CLI provider to avoid API key env, got %q", profile.APIKeyEnv)
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

func TestSetupRejectsRawSecretAsAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	output, err := executeRootForTest(
		"setup",
		"--config", dir+"/nomici.yaml",
		"--db-path", dir+"/state.db",
		"--provider", "openai",
		"--model", "gpt-4.1",
		"--api-key-env", "sk-proj-secret",
		"--yes",
	)
	if err == nil {
		t.Fatalf("expected setup to reject raw secret, got output:\n%s", output)
	}
	if !strings.Contains(err.Error(), "not a raw secret") {
		t.Fatalf("expected raw secret error, got %v", err)
	}
}

func TestProviderModelsCommandUsesLiveCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"data":[{"id":"alpha-model","object":"model","owned_by":"test"}]}`))
	}))
	defer server.Close()

	output, err := executeRootForTest(
		"provider", "models", "openai",
		"--base-url", server.URL,
		"--search", "alpha",
	)
	if err != nil {
		t.Fatalf("provider models failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "alpha-model") {
		t.Fatalf("expected live model in output, got:\n%s", output)
	}
}

func TestDevRequiresSetupConfig(t *testing.T) {
	dir := t.TempDir()
	output, err := executeRootForTest(
		"dev",
		"--config", filepath.Join(dir, "missing.yaml"),
		"--db-path", filepath.Join(dir, "state.db"),
		"--no-open",
	)
	if err == nil {
		t.Fatalf("expected dev to require config, got output:\n%s", output)
	}
	if !strings.Contains(err.Error(), "run `nomici setup` first") {
		t.Fatalf("expected setup remediation, got %v", err)
	}
}

func installFakeCodexCLI(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	name := "codex"
	content := "#!/bin/sh\nexit 0\n"
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name = "codex.bat"
		content = "@echo off\r\nexit /B 0\r\n"
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(binDir, name), []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
