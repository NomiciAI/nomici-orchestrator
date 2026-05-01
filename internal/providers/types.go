package providers

import (
	"fmt"
	"strings"
)

const (
	KindOpenAICompatible = "openai_compatible"
	KindOllama           = "ollama"
)

type Profile struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	BaseURL         string            `json:"base_url"`
	Model           string            `json:"model"`
	APIKeyEnv       string            `json:"api_key_env"`
	Capabilities    map[string]string `json:"capabilities"`
	ContextWindow   int               `json:"context_window"`
	CostPer1MInput  float64           `json:"cost_per_1m_input,omitempty"`
	CostPer1MOutput float64           `json:"cost_per_1m_output,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

func (profile *Profile) Validate() error {
	if strings.TrimSpace(profile.ID) == "" {
		return fmt.Errorf("provider id is required")
	}
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("provider name is required")
	}
	if strings.TrimSpace(profile.Kind) == "" {
		return fmt.Errorf("provider kind is required")
	}
	if !KnownKind(profile.Kind) {
		return fmt.Errorf("unsupported provider kind %q", profile.Kind)
	}
	if strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("provider model is required")
	}
	if strings.TrimSpace(profile.BaseURL) == "" {
		return fmt.Errorf("provider base_url is required")
	}
	if profile.Kind == KindOpenAICompatible && strings.TrimSpace(profile.APIKeyEnv) == "" {
		return fmt.Errorf("api_key_env is required for openai_compatible providers")
	}
	return nil
}

func KnownKind(kind string) bool {
	switch NormalizeKind(kind) {
	case KindOpenAICompatible, KindOllama:
		return true
	default:
		return false
	}
}

func NormalizeKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "openai-compatible", "openai", "openai_compatible":
		return KindOpenAICompatible
	default:
		return kind
	}
}

func DefaultBaseURL(kind string) string {
	switch NormalizeKind(kind) {
	case KindOllama:
		return "http://127.0.0.1:11434/v1"
	case KindOpenAICompatible:
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}
