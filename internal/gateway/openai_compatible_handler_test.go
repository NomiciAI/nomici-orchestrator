package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

func TestV1ModelsAndChatCompletions(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")

	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer sk-test-secret" {
			t.Fatalf("expected provider bearer auth, got %q", got)
		}
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Hello through v1."}}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
		}`))
	}))
	defer providerServer.Close()

	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	providerStore := providers.NewStore(db)
	if err := providerStore.Save(context.Background(), &providers.Profile{
		ID:        "gpt",
		Name:      "gpt",
		Kind:      providers.KindOpenAICompatible,
		BaseURL:   providerServer.URL,
		Model:     "test-model",
		APIKeyEnv: "NOMICI_TEST_API_KEY",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	router := NewRouter(Options{Version: "test", AuthToken: "gateway-token"}, Services{
		Providers: providerStore,
		Trace:     trace.NewStore(db),
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewModelAdapter(),
	})

	models := httptest.NewRecorder()
	modelsRequest := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	modelsRequest.Header.Set("Authorization", "Bearer gateway-token")
	router.ServeHTTP(models, modelsRequest)
	if models.Code != http.StatusOK {
		t.Fatalf("expected models status 200, got %d: %s", models.Code, models.Body.String())
	}
	if !strings.Contains(models.Body.String(), `"id":"gpt"`) {
		t.Fatalf("expected model profile in response, got %s", models.Body.String())
	}

	chat := httptest.NewRecorder()
	chatRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{
		"model": "gpt",
		"messages": [{"role": "user", "content": "Say hello"}]
	}`))
	chatRequest.Header.Set("Authorization", "Bearer gateway-token")
	router.ServeHTTP(chat, chatRequest)
	if chat.Code != http.StatusOK {
		t.Fatalf("expected chat status 200, got %d: %s", chat.Code, chat.Body.String())
	}
	if strings.Contains(chat.Body.String(), "sk-test-secret") {
		t.Fatal("v1 response leaked raw provider secret")
	}

	var response struct {
		Object  string `json:"object"`
		Choices []struct {
			Message adapters.Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(chat.Body).Decode(&response); err != nil {
		t.Fatalf("decode chat response: %v", err)
	}
	if response.Object != "chat.completion" {
		t.Fatalf("unexpected object %q", response.Object)
	}
	if response.Choices[0].Message.Content != "Hello through v1." {
		t.Fatalf("unexpected message: %+v", response.Choices)
	}
	if response.Usage.TotalTokens != 8 {
		t.Fatalf("expected 8 total tokens, got %d", response.Usage.TotalTokens)
	}
}

func TestV1ChatCompletionsRequiresGatewayToken(t *testing.T) {
	router := NewRouter(Options{Version: "test", AuthToken: "gateway-token"}, Services{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", response.Code, response.Body.String())
	}
}
