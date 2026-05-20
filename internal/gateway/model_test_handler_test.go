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

func TestModelTestEndpointIntegration(t *testing.T) {
	t.Setenv("NOMICI_TEST_API_KEY", "sk-test-secret")

	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer sk-test-secret" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		_, _ = response.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "Hello from test."}}],
			"usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}
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

	router := NewRouter(Options{Version: "test"}, Services{
		Providers: providerStore,
		Trace:     trace.NewStore(db),
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewModelAdapter(),
	})

	body := bytes.NewBufferString(`{"provider_id":"gpt","prompt":"Say hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/models/test", body)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sk-test-secret") {
		t.Fatal("response leaked raw secret")
	}

	var envelope struct {
		Data struct {
			RunID           string             `json:"run_id"`
			Status          string             `json:"status"`
			Messages        []adapters.Message `json:"messages"`
			TraceEventCount int                `json:"trace_event_count"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Status != adapters.StatusCompleted {
		t.Fatalf("expected completed, got %q", envelope.Data.Status)
	}
	if envelope.Data.Messages[0].Content != "Hello from test." {
		t.Fatalf("unexpected message: %+v", envelope.Data.Messages)
	}
	if envelope.Data.TraceEventCount != 4 {
		t.Fatalf("expected four trace events, got %d", envelope.Data.TraceEventCount)
	}

	events, err := trace.NewStore(db).ListByRun(context.Background(), envelope.Data.RunID)
	if err != nil {
		t.Fatalf("list trace events: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected four trace events, got %d", len(events))
	}
	for _, event := range events {
		if strings.Contains(string(event.Payload), "sk-test-secret") {
			t.Fatalf("trace event %s leaked raw secret", event.EventID)
		}
	}
}

func TestModelTestEndpointMissingSecret(t *testing.T) {
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
		BaseURL:   "http://127.0.0.1:1/v1",
		Model:     "test-model",
		APIKeyEnv: "NOMICI_MISSING_KEY",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	router := NewRouter(Options{Version: "test"}, Services{
		Providers: providerStore,
		Trace:     trace.NewStore(db),
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewModelAdapter(),
	})

	request := httptest.NewRequest(http.MethodPost, "/api/models/test", bytes.NewBufferString(`{"provider_id":"gpt","prompt":"Say hello"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "missing_secret") {
		t.Fatalf("expected missing_secret error, got %s", response.Body.String())
	}
}
