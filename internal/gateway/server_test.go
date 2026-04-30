package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	NewRouter(Options{Version: "test"}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var health HealthResponse
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if health.Status != "ok" {
		t.Fatalf("expected status ok, got %q", health.Status)
	}
	if health.Service != "nomici-gateway" {
		t.Fatalf("expected service nomici-gateway, got %q", health.Service)
	}
	if health.Version != "test" {
		t.Fatalf("expected version test, got %q", health.Version)
	}
}
