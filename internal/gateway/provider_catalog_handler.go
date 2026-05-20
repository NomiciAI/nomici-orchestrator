package gateway

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/go-chi/chi/v5"
)

type providerDoctorRequest struct {
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
}

type providerDoctorResponse struct {
	ProviderID string `json:"provider_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

func providerCatalogHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), providers.ProviderCatalog(), nil)
	}
}

func providerCatalogModelsHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		providerID := chi.URLParam(request, "provider_id")
		provider, ok := providers.GetProviderDefinition(providerID)
		if !ok {
			writeError(response, http.StatusNotFound, requestID, "provider_not_found", "Provider was not found.", "Run `nomici provider list`.")
			return
		}
		baseURL := request.URL.Query().Get("base_url")
		if baseURL == "" {
			baseURL = provider.DefaultBaseURL
		}
		apiKeyEnv := request.URL.Query().Get("api_key_env")
		if apiKeyEnv == "" {
			apiKeyEnv = provider.DefaultAPIKeyEnv
		}
		apiKey := ""
		if apiKeyEnv != "" {
			apiKey = os.Getenv(apiKeyEnv)
		}
		result, err := (providers.ModelCatalogClient{}).ListModels(request.Context(), providers.ModelCatalogRequest{
			ProviderID: provider.ID,
			BaseURL:    baseURL,
			APIKey:     apiKey,
			Query:      request.URL.Query().Get("q"),
		})
		if err != nil {
			writeError(response, http.StatusBadGateway, requestID, "model_catalog_failed", err.Error(), "Check provider credentials, base URL, or use a custom model id.")
			return
		}
		var warnings []string
		if result.Message != "" {
			warnings = append(warnings, result.Message)
		}
		writeSuccess(response, requestID, result, warnings)
	}
}

func providerCatalogDoctorHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		providerID := chi.URLParam(request, "provider_id")
		provider, ok := providers.GetProviderDefinition(providerID)
		if !ok {
			writeError(response, http.StatusNotFound, requestID, "provider_not_found", "Provider was not found.", "Run `nomici provider list`.")
			return
		}
		var body providerDoctorRequest
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		baseURL := body.BaseURL
		if baseURL == "" {
			baseURL = provider.DefaultBaseURL
		}
		apiKeyEnv := body.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = provider.DefaultAPIKeyEnv
		}
		if provider.Local {
			status := "ok"
			if !provider.Available {
				status = "failed"
			}
			writeSuccess(response, requestID, providerDoctorResponse{
				ProviderID: provider.ID,
				Status:     status,
				Message:    provider.AvailabilityMessage,
			}, nil)
			return
		}
		apiKey := ""
		if apiKeyEnv != "" {
			if value, ok := os.LookupEnv(apiKeyEnv); ok {
				apiKey = value
			} else if provider.AuthMode == providers.AuthModeAPIKeyEnv {
				writeSuccess(response, requestID, providerDoctorResponse{
					ProviderID: provider.ID,
					Status:     "warning",
					Message:    apiKeyEnv + " is not set",
				}, nil)
				return
			}
		}
		catalog, err := (providers.ModelCatalogClient{}).ListModels(request.Context(), providers.ModelCatalogRequest{
			ProviderID: provider.ID,
			BaseURL:    baseURL,
			APIKey:     apiKey,
		})
		if err != nil {
			writeSuccess(response, requestID, providerDoctorResponse{
				ProviderID: provider.ID,
				Status:     "warning",
				Message:    err.Error(),
			}, nil)
			return
		}
		writeSuccess(response, requestID, providerDoctorResponse{
			ProviderID: provider.ID,
			Status:     "ok",
			Message:    catalog.Source,
		}, nil)
	}
}
