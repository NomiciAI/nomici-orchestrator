package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderOpenAI                = "openai"
	ProviderAnthropic             = "anthropic"
	ProviderGemini                = "gemini"
	ProviderDeepSeek              = "deepseek"
	ProviderOpenRouter            = "openrouter"
	ProviderVLLM                  = "vllm"
	ProviderOllama                = "ollama"
	ProviderCodexCLI              = "codex_cli"
	ProviderClaudeCodeOAuth       = "claude_code_oauth"
	ProviderOtherOpenAICompatible = "other_openai_compatible"

	AuthModeAPIKeyEnv = "api_key_env"
	AuthModeLocal     = "local_auth"
	AuthModeNone      = "none"
)

type ProviderDefinition struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	AdapterKind         string         `json:"adapter_kind"`
	Description         string         `json:"description"`
	DefaultBaseURL      string         `json:"default_base_url"`
	DefaultAPIKeyEnv    string         `json:"default_api_key_env,omitempty"`
	AuthMode            string         `json:"auth_mode"`
	CatalogMode         string         `json:"catalog_mode"`
	SupportsCustomModel bool           `json:"supports_custom_model"`
	RequiresBaseURL     bool           `json:"requires_base_url"`
	Local               bool           `json:"local"`
	RecommendedModels   []ModelSummary `json:"recommended_models"`
	Available           bool           `json:"available"`
	AvailabilityMessage string         `json:"availability_message,omitempty"`
}

type ModelSummary struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	ProviderID       string            `json:"provider_id"`
	Source           string            `json:"source"`
	OwnedBy          string            `json:"owned_by,omitempty"`
	Created          int64             `json:"created,omitempty"`
	ContextWindow    int               `json:"context_window,omitempty"`
	InputModalities  []string          `json:"input_modalities,omitempty"`
	OutputModalities []string          `json:"output_modalities,omitempty"`
	Pricing          map[string]string `json:"pricing,omitempty"`
	Recommended      bool              `json:"recommended,omitempty"`
}

type ModelCatalogRequest struct {
	ProviderID string
	BaseURL    string
	APIKey     string
	Query      string
}

type ModelCatalogResult struct {
	ProviderID string         `json:"provider_id"`
	Source     string         `json:"source"`
	Cached     bool           `json:"cached"`
	Models     []ModelSummary `json:"models"`
	Message    string         `json:"message,omitempty"`
	CheckedAt  time.Time      `json:"checked_at"`
}

type ModelCatalogClient struct {
	HTTPClient *http.Client
}

func ProviderCatalog() []ProviderDefinition {
	catalog := []ProviderDefinition{
		{
			ID:                  ProviderOpenAI,
			Name:                "OpenAI",
			AdapterKind:         KindOpenAICompatible,
			Description:         "OpenAI API with account-scoped model catalog",
			DefaultBaseURL:      "https://api.openai.com/v1",
			DefaultAPIKeyEnv:    "OPENAI_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "openai_models",
			SupportsCustomModel: true,
			RecommendedModels: recommended(ProviderOpenAI,
				"gpt-5.4",
				"gpt-5.4-mini",
				"gpt-4.1",
				"gpt-4o",
				"o3",
			),
		},
		{
			ID:                  ProviderAnthropic,
			Name:                "Anthropic",
			AdapterKind:         KindAnthropic,
			Description:         "Anthropic Messages API with account-scoped model catalog",
			DefaultBaseURL:      "https://api.anthropic.com",
			DefaultAPIKeyEnv:    "ANTHROPIC_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "anthropic_models",
			SupportsCustomModel: true,
			RecommendedModels: recommended(ProviderAnthropic,
				"claude-opus-4-1-20250805",
				"claude-opus-4-20250514",
				"claude-sonnet-4-20250514",
				"claude-3-7-sonnet-20250219",
			),
		},
		{
			ID:                  ProviderGemini,
			Name:                "Google Gemini",
			AdapterKind:         KindGemini,
			Description:         "Gemini API with programmatic model discovery",
			DefaultBaseURL:      "https://generativelanguage.googleapis.com/v1beta",
			DefaultAPIKeyEnv:    "GEMINI_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "gemini_models",
			SupportsCustomModel: true,
			RecommendedModels: recommended(ProviderGemini,
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-2.0-flash",
			),
		},
		{
			ID:                  ProviderDeepSeek,
			Name:                "DeepSeek",
			AdapterKind:         KindOpenAICompatible,
			Description:         "DeepSeek OpenAI-compatible API with provider model catalog",
			DefaultBaseURL:      "https://api.deepseek.com/v1",
			DefaultAPIKeyEnv:    "DEEPSEEK_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "openai_models",
			SupportsCustomModel: true,
			RecommendedModels: recommended(ProviderDeepSeek,
				"deepseek-v4-pro",
				"deepseek-v4-flash",
				"deepseek-chat",
				"deepseek-reasoner",
			),
		},
		{
			ID:                  ProviderOpenRouter,
			Name:                "OpenRouter",
			AdapterKind:         KindOpenAICompatible,
			Description:         "OpenAI-compatible gateway with a broad live model catalog",
			DefaultBaseURL:      "https://openrouter.ai/api/v1",
			DefaultAPIKeyEnv:    "OPENROUTER_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "openrouter_models",
			SupportsCustomModel: true,
			RecommendedModels: recommended(ProviderOpenRouter,
				"openai/gpt-5.4",
				"anthropic/claude-sonnet-4",
				"google/gemini-2.5-pro",
				"deepseek/deepseek-r1",
			),
		},
		{
			ID:                  ProviderVLLM,
			Name:                "vLLM",
			AdapterKind:         KindOpenAICompatible,
			Description:         "Self-hosted OpenAI-compatible serving",
			DefaultBaseURL:      "http://127.0.0.1:8000/v1",
			AuthMode:            AuthModeNone,
			CatalogMode:         "openai_models",
			SupportsCustomModel: true,
			RequiresBaseURL:     true,
			RecommendedModels: recommended(ProviderVLLM,
				"NousResearch/Meta-Llama-3-8B-Instruct",
			),
		},
		{
			ID:                  ProviderOllama,
			Name:                "Ollama",
			AdapterKind:         KindOllama,
			Description:         "Local Ollama server using its OpenAI-compatible endpoint",
			DefaultBaseURL:      "http://127.0.0.1:11434/v1",
			AuthMode:            AuthModeNone,
			CatalogMode:         "openai_models",
			SupportsCustomModel: true,
			RequiresBaseURL:     true,
			Local:               true,
			RecommendedModels: recommended(ProviderOllama,
				"llama3.2",
				"qwen2.5-coder",
				"mistral",
			),
		},
		{
			ID:                  ProviderCodexCLI,
			Name:                "Codex CLI",
			AdapterKind:         KindCodexCLI,
			Description:         "Uses local Codex CLI authentication",
			DefaultBaseURL:      "local://codex-cli",
			AuthMode:            AuthModeLocal,
			CatalogMode:         "local_cli",
			SupportsCustomModel: true,
			Local:               true,
			RecommendedModels: recommended(ProviderCodexCLI,
				"gpt-5.4",
				"gpt-5.4-mini",
			),
		},
		{
			ID:                  ProviderClaudeCodeOAuth,
			Name:                "Claude Code OAuth",
			AdapterKind:         KindClaudeCode,
			Description:         "Uses local Claude Code credentials",
			DefaultBaseURL:      "local://claude-code",
			AuthMode:            AuthModeLocal,
			CatalogMode:         "local_cli",
			SupportsCustomModel: true,
			Local:               true,
			RecommendedModels: recommended(ProviderClaudeCodeOAuth,
				"sonnet",
				"opus",
			),
		},
		{
			ID:                  ProviderOtherOpenAICompatible,
			Name:                "Other OpenAI-compatible",
			AdapterKind:         KindOpenAICompatible,
			Description:         "Custom gateway with base_url and model name",
			DefaultBaseURL:      "https://api.openai.com/v1",
			DefaultAPIKeyEnv:    "OPENAI_API_KEY",
			AuthMode:            AuthModeAPIKeyEnv,
			CatalogMode:         "openai_models",
			SupportsCustomModel: true,
			RequiresBaseURL:     true,
			RecommendedModels:   []ModelSummary{},
		},
	}
	for index := range catalog {
		switch catalog[index].ID {
		case ProviderCodexCLI:
			availability := DetectCodexCLI()
			catalog[index].Available = availability.Available
			catalog[index].AvailabilityMessage = availability.Message
		case ProviderClaudeCodeOAuth:
			availability := DetectClaudeCode()
			catalog[index].Available = availability.Available
			catalog[index].AvailabilityMessage = availability.Message
		default:
			catalog[index].Available = true
		}
	}
	return catalog
}

func GetProviderDefinition(providerID string) (ProviderDefinition, bool) {
	normalized := NormalizeProviderID(providerID)
	for _, provider := range ProviderCatalog() {
		if provider.ID == normalized {
			return provider, true
		}
	}
	return ProviderDefinition{}, false
}

func NormalizeProviderID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "openai_compatible", "openai-compatible":
		return ProviderOpenAI
	case "google", "google_gemini":
		return ProviderGemini
	case "open_router":
		return ProviderOpenRouter
	case "codex", "codex_cli":
		return ProviderCodexCLI
	case "claude", "claude_code", "claude_code_oauth":
		return ProviderClaudeCodeOAuth
	case "other", "custom", "custom_openai_compatible":
		return ProviderOtherOpenAICompatible
	default:
		return value
	}
}

func (client ModelCatalogClient) ListModels(ctx context.Context, request ModelCatalogRequest) (*ModelCatalogResult, error) {
	provider, ok := GetProviderDefinition(request.ProviderID)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", request.ProviderID)
	}
	baseURL := strings.TrimSpace(request.BaseURL)
	if baseURL == "" {
		baseURL = provider.DefaultBaseURL
	}
	checkedAt := time.Now().UTC()
	var models []ModelSummary
	var err error
	switch provider.CatalogMode {
	case "openrouter_models":
		models, err = client.listOpenRouterModels(ctx, provider.ID, baseURL, request.APIKey)
	case "anthropic_models":
		models, err = client.listAnthropicModels(ctx, provider.ID, baseURL, request.APIKey)
	case "gemini_models":
		models, err = client.listGeminiModels(ctx, provider.ID, baseURL, request.APIKey)
	case "openai_models":
		models, err = client.listOpenAIModels(ctx, provider.ID, baseURL, request.APIKey)
	case "local_cli":
		models = provider.RecommendedModels
	default:
		err = fmt.Errorf("unsupported model catalog mode %q", provider.CatalogMode)
	}
	if err != nil {
		if len(provider.RecommendedModels) == 0 {
			return nil, err
		}
		models = provider.RecommendedModels
		return &ModelCatalogResult{
			ProviderID: provider.ID,
			Source:     "recommended_fallback",
			Models:     filterModels(models, request.Query),
			Message:    err.Error(),
			CheckedAt:  checkedAt,
		}, nil
	}
	models = markRecommended(provider.RecommendedModels, models)
	models = filterModels(models, request.Query)
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Recommended != models[j].Recommended {
			return models[i].Recommended
		}
		return strings.ToLower(models[i].ID) < strings.ToLower(models[j].ID)
	})
	return &ModelCatalogResult{
		ProviderID: provider.ID,
		Source:     provider.CatalogMode,
		Models:     models,
		CheckedAt:  checkedAt,
	}, nil
}

func (client ModelCatalogClient) listOpenAIModels(ctx context.Context, providerID string, baseURL string, apiKey string) ([]ModelSummary, error) {
	var decoded struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := client.getJSON(ctx, strings.TrimRight(baseURL, "/")+"/models", apiKey, nil, &decoded); err != nil {
		return nil, err
	}
	models := make([]ModelSummary, 0, len(decoded.Data))
	for _, model := range decoded.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		models = append(models, ModelSummary{
			ID:         model.ID,
			Name:       model.ID,
			ProviderID: providerID,
			Source:     "provider_api",
			OwnedBy:    model.OwnedBy,
			Created:    model.Created,
		})
	}
	return models, nil
}

func (client ModelCatalogClient) listAnthropicModels(ctx context.Context, providerID string, baseURL string, apiKey string) ([]ModelSummary, error) {
	var decoded struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
			Type        string `json:"type"`
		} `json:"data"`
	}
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	if err := client.getJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/models", apiKey, headers, &decoded); err != nil {
		return nil, err
	}
	models := make([]ModelSummary, 0, len(decoded.Data))
	for _, model := range decoded.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		models = append(models, ModelSummary{
			ID:         model.ID,
			Name:       model.DisplayName,
			ProviderID: providerID,
			Source:     "provider_api",
			OwnedBy:    "anthropic",
			Created:    parseRFC3339Unix(model.CreatedAt),
		})
	}
	return models, nil
}

func (client ModelCatalogClient) listGeminiModels(ctx context.Context, providerID string, baseURL string, apiKey string) ([]ModelSummary, error) {
	var decoded struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Version                    string   `json:"version"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/models"
	if strings.TrimSpace(apiKey) != "" {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		endpoint += separator + "key=" + url.QueryEscape(apiKey)
	}
	if err := client.getJSON(ctx, endpoint, "", nil, &decoded); err != nil {
		return nil, err
	}
	models := make([]ModelSummary, 0, len(decoded.Models))
	for _, model := range decoded.Models {
		id := strings.TrimPrefix(model.Name, "models/")
		if id == "" {
			continue
		}
		models = append(models, ModelSummary{
			ID:            id,
			Name:          model.DisplayName,
			ProviderID:    providerID,
			Source:        "provider_api",
			OwnedBy:       "google",
			ContextWindow: model.InputTokenLimit,
			OutputModalities: append([]string{},
				model.SupportedGenerationMethods...,
			),
		})
	}
	return models, nil
}

func (client ModelCatalogClient) listOpenRouterModels(ctx context.Context, providerID string, baseURL string, apiKey string) ([]ModelSummary, error) {
	var decoded struct {
		Data []struct {
			ID            string            `json:"id"`
			Name          string            `json:"name"`
			Created       int64             `json:"created"`
			ContextLength int               `json:"context_length"`
			Pricing       map[string]string `json:"pricing"`
			Architecture  struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			TopProvider struct {
				ContextLength int `json:"context_length"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := client.getJSON(ctx, strings.TrimRight(baseURL, "/")+"/models", apiKey, nil, &decoded); err != nil {
		return nil, err
	}
	models := make([]ModelSummary, 0, len(decoded.Data))
	for _, model := range decoded.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		contextWindow := model.ContextLength
		if contextWindow == 0 {
			contextWindow = model.TopProvider.ContextLength
		}
		models = append(models, ModelSummary{
			ID:               model.ID,
			Name:             model.Name,
			ProviderID:       providerID,
			Source:           "provider_api",
			Created:          model.Created,
			ContextWindow:    contextWindow,
			InputModalities:  model.Architecture.InputModalities,
			OutputModalities: model.Architecture.OutputModalities,
			Pricing:          model.Pricing,
		})
	}
	return models, nil
}

func (client ModelCatalogClient) getJSON(ctx context.Context, endpoint string, apiKey string, headers map[string]string, target any) error {
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create model catalog request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("x-api-key", apiKey)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("model catalog request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read model catalog response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("model catalog returned HTTP %d", response.StatusCode)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode model catalog response: %w", err)
	}
	return nil
}

func recommended(providerID string, modelIDs ...string) []ModelSummary {
	models := make([]ModelSummary, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		models = append(models, ModelSummary{
			ID:          modelID,
			Name:        modelID,
			ProviderID:  providerID,
			Source:      "recommended",
			Recommended: true,
		})
	}
	return models
}

func markRecommended(recommendedModels []ModelSummary, models []ModelSummary) []ModelSummary {
	recommendedIDs := map[string]bool{}
	for _, model := range recommendedModels {
		recommendedIDs[strings.ToLower(model.ID)] = true
	}
	for index := range models {
		if recommendedIDs[strings.ToLower(models[index].ID)] {
			models[index].Recommended = true
		}
	}
	return models
}

func filterModels(models []ModelSummary, query string) []ModelSummary {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return models
	}
	filtered := make([]ModelSummary, 0, len(models))
	for _, model := range models {
		search := strings.ToLower(model.ID + " " + model.Name + " " + model.OwnedBy + " " + strings.Join(model.InputModalities, " ") + " " + strings.Join(model.OutputModalities, " "))
		if strings.Contains(search, query) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func parseRFC3339Unix(value string) int64 {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.Unix()
	}
	if numeric, err := strconv.ParseInt(value, 10, 64); err == nil {
		return numeric
	}
	return 0
}
