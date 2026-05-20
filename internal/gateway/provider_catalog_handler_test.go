package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProviderCatalogEndpoints(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"data":[{"id":"catalog-model","object":"model","owned_by":"test"}]}`))
	}))
	defer providerServer.Close()

	router := NewRouter(Options{Version: "test"}, Services{})

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/provider-catalog", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected provider catalog 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), "openrouter") || !strings.Contains(listResponse.Body.String(), "gemini") {
		t.Fatalf("expected provider ids, got %s", listResponse.Body.String())
	}

	modelsResponse := httptest.NewRecorder()
	router.ServeHTTP(modelsResponse, httptest.NewRequest(http.MethodGet, "/api/provider-catalog/openai/models?base_url="+url.QueryEscape(providerServer.URL+"/v1")+"&q=catalog", nil))
	if modelsResponse.Code != http.StatusOK {
		t.Fatalf("expected models 200, got %d: %s", modelsResponse.Code, modelsResponse.Body.String())
	}
	var envelope struct {
		Data struct {
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"data"`
	}
	if err := json.NewDecoder(modelsResponse.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Models) != 1 || envelope.Data.Models[0].ID != "catalog-model" {
		t.Fatalf("unexpected models response: %+v", envelope.Data.Models)
	}
}
