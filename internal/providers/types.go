package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Available          bool
	Executable         string
	ExecutableSource   string
	CheckedExecutables []string
	AuthPath           string
	AuthSource         string
	OS                 string
	Arch               string
	Message            string
}

func DetectCodexCLI() CodexCLIAvailability {
	authPath, authSource := CodexAuthPathWithSource()
	executable, executableSource, checkedExecutables := ResolveCodexCLIExecutable()
	availability := CodexCLIAvailability{
		Executable:         executable,
		ExecutableSource:   executableSource,
		CheckedExecutables: checkedExecutables,
		AuthPath:           authPath,
		AuthSource:         authSource,
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
	}
	if executable == "" {
		availability.Message = fmt.Sprintf("codex executable was not found for %s/%s; checked %s", availability.OS, availability.Arch, strings.Join(checkedExecutables, ", "))
		return availability
	}
	if _, err := os.Stat(authPath); err != nil {
		if os.IsNotExist(err) {
			availability.Message = fmt.Sprintf("Codex CLI local auth was not found at %s (%s); executable resolved at %s (%s)", authPath, authSource, executable, executableSource)
			return availability
		}
		availability.Message = fmt.Sprintf("Codex CLI local auth at %s (%s) could not be checked: %v", authPath, authSource, err)
		return availability
	}
	availability.Available = true
	availability.Message = fmt.Sprintf("Codex CLI local auth available at %s (%s); executable=%s (%s) for %s/%s", authPath, authSource, executable, executableSource, availability.OS, availability.Arch)
	return availability
}

func ResolveCodexCLIExecutable() (string, string, []string) {
	checked := CodexCLIExecutableCandidates()
	if executable, err := exec.LookPath("codex"); err == nil {
		return executable, "PATH", checked
	}
	for _, candidate := range checked {
		if candidate == "PATH:codex" {
			continue
		}
		if fileIsRunnable(candidate) {
			return candidate, "app bundle", checked
		}
	}
	return "", "", checked
}

func CodexCLIExecutableCandidates() []string {
	candidates := []string{"PATH:codex"}
	if runtime.GOOS != "darwin" {
		return candidates
	}
	if override, ok := os.LookupEnv("NOMICI_CODEX_APP_EXECUTABLES"); ok {
		for _, candidate := range filepath.SplitList(override) {
			if candidate = strings.TrimSpace(candidate); candidate != "" {
				candidates = append(candidates, candidate)
			}
		}
		return candidates
	}
	candidates = append(candidates, "/Applications/Codex.app/Contents/Resources/codex")
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		candidates = append(candidates, filepath.Join(home, "Applications", "Codex.app", "Contents", "Resources", "codex"))
	}
	return candidates
}

func fileIsRunnable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func CodexAuthPath() string {
	path, _ := CodexAuthPathWithSource()
	return path
}

func CodexAuthPathWithSource() (string, string) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Join(home, "auth.json"), "CODEX_HOME"
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".codex", "auth.json"), "user home"
	}
	return filepath.Join(".codex", "auth.json"), "relative fallback"
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
