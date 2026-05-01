package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayAuthProtectsAPIExceptHealth(t *testing.T) {
	router := NewRouter(Options{Version: "test", AuthToken: "test-token"}, Services{})

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("expected health to remain public, got %d", health.Code)
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing token to return 401, got %d", missing.Code)
	}
	if !strings.Contains(missing.Body.String(), "unauthorized") {
		t.Fatalf("expected unauthorized envelope, got %s", missing.Body.String())
	}

	bad := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	badRequest.Header.Set("Authorization", "Bearer wrong-token")
	router.ServeHTTP(bad, badRequest)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected bad token to return 401, got %d", bad.Code)
	}

	good := httptest.NewRecorder()
	goodRequest := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	goodRequest.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(good, goodRequest)
	if good.Code == http.StatusUnauthorized {
		t.Fatalf("expected valid token to pass auth, got %d: %s", good.Code, good.Body.String())
	}
}
