package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	KindOpenAICompatible = "openai_compatible"
	KindAnthropic        = "anthropic"
	KindGemini           = "gemini"
	KindOllama           = "ollama"
	KindCodexCLI         = "codex_cli"
	KindClaudeCode       = "claude_code"
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
	if profile.RequiresAPIKey() && strings.TrimSpace(profile.APIKeyEnv) == "" {
		return fmt.Errorf("api_key_env is required for %s providers", NormalizeKind(profile.Kind))
	}
	return nil
}

func (profile *Profile) RequiresAPIKey() bool {
	if profile == nil {
		return false
	}
	kind := NormalizeKind(profile.Kind)
	switch kind {
	case KindAnthropic, KindGemini:
		return true
	case KindOpenAICompatible:
		if profile.Capabilities != nil {
			if profile.Capabilities["auth_mode"] == AuthModeNone {
				return false
			}
			switch profile.Capabilities["provider_id"] {
			case ProviderVLLM, ProviderOllama:
				return false
			}
		}
		return true
	default:
		return false
	}
}

func KnownKind(kind string) bool {
	switch NormalizeKind(kind) {
	case KindOpenAICompatible, KindAnthropic, KindGemini, KindOllama, KindCodexCLI, KindClaudeCode:
		return true
	default:
		return false
	}
}

func NormalizeKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "openai-compatible", "openai", "openai_compatible", "deepseek", "openrouter", "vllm", "other_openai_compatible":
		return KindOpenAICompatible
	case "codex-cli", "codex", "codex_cli":
		return KindCodexCLI
	case "claude-code", "claude_code", "claude":
		return KindClaudeCode
	case "google-gemini", "google_gemini", "gemini":
		return KindGemini
	default:
		return kind
	}
}

func DefaultBaseURL(kind string) string {
	switch NormalizeKind(kind) {
	case KindOllama:
		return "http://127.0.0.1:11434/v1"
	case KindCodexCLI:
		return "local://codex-cli"
	case KindClaudeCode:
		return "local://claude-code"
	case KindAnthropic:
		return "https://api.anthropic.com"
	case KindGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	case KindOpenAICompatible:
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

func ProviderRequiresAPIKey(kind string) bool {
	switch NormalizeKind(kind) {
	case KindOpenAICompatible, KindAnthropic, KindGemini:
		return true
	default:
		return false
	}
}

type CodexCLIAvailability struct {
	Available  bool
	Executable string
	AuthPath   string
	Message    string
}

func DetectCodexCLI() CodexCLIAvailability {
	executable, err := exec.LookPath("codex")
	if err != nil {
		return CodexCLIAvailability{Available: false, AuthPath: CodexAuthPath(), Message: "codex executable was not found on PATH"}
	}
	authPath := CodexAuthPath()
	if _, err := os.Stat(authPath); err != nil {
		if os.IsNotExist(err) {
			return CodexCLIAvailability{Available: false, Executable: executable, AuthPath: authPath, Message: "Codex CLI local auth was not found"}
		}
		return CodexCLIAvailability{Available: false, Executable: executable, AuthPath: authPath, Message: "Codex CLI local auth could not be checked"}
	}
	return CodexCLIAvailability{Available: true, Executable: executable, AuthPath: authPath, Message: "Codex CLI local auth available"}
}

func CodexAuthPath() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Join(home, "auth.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".codex", "auth.json")
	}
	return filepath.Join(".codex", "auth.json")
}

type ClaudeCodeAvailability struct {
	Available  bool
	Executable string
	Message    string
}

func DetectClaudeCode() ClaudeCodeAvailability {
	executable, err := exec.LookPath("claude")
	if err != nil {
		return ClaudeCodeAvailability{Available: false, Message: "claude executable was not found on PATH"}
	}
	return ClaudeCodeAvailability{Available: true, Executable: executable, Message: "Claude Code executable available; local auth is validated by the CLI"}
}
