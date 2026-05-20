package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProviderCatalogHasUniqueEntries(t *testing.T) {
	seen := map[string]bool{}
	for _, provider := range ProviderCatalog() {
		if seen[provider.ID] {
			t.Fatalf("duplicate provider id %q", provider.ID)
		}
		seen[provider.ID] = true
		if provider.AdapterKind == "" || provider.CatalogMode == "" {
			t.Fatalf("provider missing adapter or catalog mode: %+v", provider)
		}
	}
	for _, required := range []string{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderDeepSeek, ProviderOpenRouter, ProviderVLLM, ProviderOllama, ProviderCodexCLI, ProviderClaudeCodeOAuth, ProviderOtherOpenAICompatible} {
		if !seen[required] {
			t.Fatalf("expected provider %q in catalog", required)
		}
	}
}

func TestDetectCodexCLIReportsPlatformAndAuthPath(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	installFakeCodexExecutable(t, binDir)
	codexHome := filepath.Join(dir, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CODEX_HOME", codexHome)

	availability := DetectCodexCLI()
	if availability.Available {
		t.Fatal("expected missing local auth to report unavailable")
	}
	if availability.OS != runtime.GOOS || availability.Arch != runtime.GOARCH {
		t.Fatalf("expected platform in availability, got %+v", availability)
	}
	expectedAuthPath := filepath.Join(codexHome, "auth.json")
	if availability.AuthPath != expectedAuthPath || !strings.Contains(availability.Message, expectedAuthPath) {
		t.Fatalf("expected auth path in availability message, got %+v", availability)
	}
	if availability.AuthSource != "CODEX_HOME" || !strings.Contains(availability.Message, "CODEX_HOME") {
		t.Fatalf("expected auth source in availability message, got %+v", availability)
	}
}

func TestDetectCodexCLIReadyFromPathAndAuth(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	executable := installFakeCodexExecutable(t, binDir)
	codexHome := filepath.Join(dir, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CODEX_HOME", codexHome)

	availability := DetectCodexCLI()
	if !availability.Available {
		t.Fatalf("expected ready Codex CLI, got %+v", availability)
	}
	if availability.Executable != executable || availability.ExecutableSource != "PATH" {
		t.Fatalf("expected PATH executable %q, got %+v", executable, availability)
	}
	if !strings.Contains(availability.Message, executable) {
		t.Fatalf("expected executable in message, got %q", availability.Message)
	}
}

func TestDetectCodexCLIMissingExecutableListsCandidates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("CODEX_HOME", filepath.Join(dir, "codex-home"))
	t.Setenv("NOMICI_CODEX_APP_EXECUTABLES", "")

	availability := DetectCodexCLI()
	if availability.Available || availability.Executable != "" {
		t.Fatalf("expected unavailable Codex CLI, got %+v", availability)
	}
	if !strings.Contains(availability.Message, "PATH:codex") {
		t.Fatalf("expected checked candidates in message, got %q", availability.Message)
	}
}

func TestDetectCodexCLIFallsBackToMacAppBundle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Codex.app fallback is macOS-only")
	}
	dir := t.TempDir()
	appExecutable := filepath.Join(dir, "Codex.app", "Contents", "Resources", "codex")
	if err := os.MkdirAll(filepath.Dir(appExecutable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appExecutable, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	codexHome := filepath.Join(dir, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "empty-bin"))
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("NOMICI_CODEX_APP_EXECUTABLES", appExecutable)

	availability := DetectCodexCLI()
	if !availability.Available {
		t.Fatalf("expected app bundle Codex CLI fallback to be ready, got %+v", availability)
	}
	if availability.Executable != appExecutable || availability.ExecutableSource != "app bundle" {
		t.Fatalf("expected app bundle executable, got %+v", availability)
	}
}

func installFakeCodexExecutable(t *testing.T, binDir string) string {
	t.Helper()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "codex"
	content := "#!/bin/sh\nexit 0\n"
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name = "codex.bat"
		content = "@echo off\r\nexit /B 0\r\n"
		mode = 0o644
	}
	executable := filepath.Join(binDir, name)
	if err := os.WriteFile(executable, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return executable
}

func TestModelCatalogClientNormalizesOpenAICompatibleModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"object":"list","data":[{"id":"alpha-model","object":"model","created":123,"owned_by":"test"}]}`))
	}))
	defer server.Close()

	result, err := (ModelCatalogClient{}).ListModels(context.Background(), ModelCatalogRequest{
		ProviderID: ProviderOpenAI,
		BaseURL:    server.URL + "/v1",
		Query:      "alpha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 1 || result.Models[0].ID != "alpha-model" || result.Models[0].OwnedBy != "test" {
		t.Fatalf("unexpected models: %+v", result.Models)
	}
}

func TestModelCatalogClientNormalizesNativeProviderModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/models":
			_, _ = response.Write([]byte(`{"data":[{"id":"claude-test","display_name":"Claude Test","created_at":"2025-02-19T00:00:00Z","type":"model"}]}`))
		case "/models":
			_, _ = response.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test","inputTokenLimit":100000,"supportedGenerationMethods":["generateContent"]}]}`))
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	defer server.Close()

	anthropic, err := (ModelCatalogClient{}).ListModels(context.Background(), ModelCatalogRequest{
		ProviderID: ProviderAnthropic,
		BaseURL:    server.URL,
		APIKey:     "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(anthropic.Models) != 1 || anthropic.Models[0].ID != "claude-test" {
		t.Fatalf("unexpected Anthropic models: %+v", anthropic.Models)
	}

	gemini, err := (ModelCatalogClient{}).ListModels(context.Background(), ModelCatalogRequest{
		ProviderID: ProviderGemini,
		BaseURL:    server.URL,
		APIKey:     "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gemini.Models) != 1 || gemini.Models[0].ID != "gemini-test" || gemini.Models[0].ContextWindow != 100000 {
		t.Fatalf("unexpected Gemini models: %+v", gemini.Models)
	}
}

func TestModelCatalogClientNormalizesOpenRouterMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"data":[{"id":"openai/example","name":"Example","context_length":8192,"pricing":{"prompt":"0.1"},"architecture":{"input_modalities":["text"],"output_modalities":["text"]}}]}`))
	}))
	defer server.Close()

	result, err := (ModelCatalogClient{}).ListModels(context.Background(), ModelCatalogRequest{
		ProviderID: ProviderOpenRouter,
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 1 || result.Models[0].ContextWindow != 8192 || result.Models[0].Pricing["prompt"] != "0.1" {
		t.Fatalf("unexpected OpenRouter models: %+v", result.Models)
	}
}
