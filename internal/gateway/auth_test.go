package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
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

func TestGatewayBootstrapAuthExchangesSingleUseToken(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	bootstrap, err := gatewayauth.CreateBootstrap(
		gatewayauth.BootstrapPathForDB(dbPath),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create bootstrap token: %v", err)
	}
	router := NewRouter(Options{Version: "test", AuthToken: "gateway-token", DBPath: dbPath}, Services{})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/auth/bootstrap", bytes.NewBufferString(`{"token":"`+bootstrap.Token+`"}`)))
	if first.Code != http.StatusOK {
		t.Fatalf("expected bootstrap exchange 200, got %d: %s", first.Code, first.Body.String())
	}
	var envelope struct {
		Data struct {
			GatewayToken string `json:"gateway_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(first.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode bootstrap response: %v", err)
	}
	if envelope.Data.GatewayToken != "gateway-token" {
		t.Fatalf("expected gateway token, got %q", envelope.Data.GatewayToken)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/auth/bootstrap", bytes.NewBufferString(`{"token":"`+bootstrap.Token+`"}`)))
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("expected second bootstrap exchange to fail, got %d: %s", second.Code, second.Body.String())
	}
}

func TestGatewayLocalReconnectRejectsCrossSiteRequests(t *testing.T) {
	router := NewRouter(Options{Version: "test", AuthToken: "gateway-token"}, Services{})

	crossSite := httptest.NewRecorder()
	crossSiteRequest := httptest.NewRequest(http.MethodPost, "/api/auth/reconnect", nil)
	crossSiteRequest.Host = "127.0.0.1:8787"
	crossSiteRequest.Header.Set("Origin", "https://example.test")
	crossSiteRequest.Header.Set("Sec-Fetch-Site", "cross-site")
	router.ServeHTTP(crossSite, crossSiteRequest)
	if crossSite.Code != http.StatusForbidden {
		t.Fatalf("expected cross-site reconnect to fail, got %d: %s", crossSite.Code, crossSite.Body.String())
	}

	sameOrigin := httptest.NewRecorder()
	sameOriginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/reconnect", nil)
	sameOriginRequest.Host = "127.0.0.1:8787"
	sameOriginRequest.Header.Set("Origin", "http://127.0.0.1:8787")
	sameOriginRequest.Header.Set("Sec-Fetch-Site", "same-origin")
	router.ServeHTTP(sameOrigin, sameOriginRequest)
	if sameOrigin.Code != http.StatusOK || !strings.Contains(sameOrigin.Body.String(), "gateway-token") {
		t.Fatalf("expected same-origin reconnect to return token, got %d: %s", sameOrigin.Code, sameOrigin.Body.String())
	}
}
