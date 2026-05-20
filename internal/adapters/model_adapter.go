package adapters

import (
	"context"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

type ModelAdapter struct {
	OpenAI     *OpenAICompatibleAdapter
	Anthropic  *AnthropicAdapter
	Gemini     *GeminiAdapter
	Codex      *CodexCLIAdapter
	ClaudeCode *ClaudeCodeAdapter
}

func NewModelAdapter() *ModelAdapter {
	return &ModelAdapter{
		OpenAI:     NewOpenAICompatibleAdapter(),
		Anthropic:  NewAnthropicAdapter(),
		Gemini:     NewGeminiAdapter(),
		Codex:      NewCodexCLIAdapter(),
		ClaudeCode: NewClaudeCodeAdapter(),
	}
}

func (adapter *ModelAdapter) Invoke(ctx context.Context, config ModelConfig, apiKey string, request InvokeRequest) (*InvokeResult, error) {
	if adapter == nil {
		return nil, fmt.Errorf("model adapter is not initialized")
	}
	switch providers.NormalizeKind(config.Kind) {
	case providers.KindCodexCLI:
		codex := adapter.Codex
		if codex == nil {
			codex = NewCodexCLIAdapter()
		}
		return codex.Invoke(ctx, config.Model, request)
	case providers.KindClaudeCode:
		claude := adapter.ClaudeCode
		if claude == nil {
			claude = NewClaudeCodeAdapter()
		}
		return claude.Invoke(ctx, config.Model, request)
	case providers.KindAnthropic:
		anthropic := adapter.Anthropic
		if anthropic == nil {
			anthropic = NewAnthropicAdapter()
		}
		return anthropic.Invoke(ctx, config.BaseURL, config.Model, apiKey, request)
	case providers.KindGemini:
		gemini := adapter.Gemini
		if gemini == nil {
			gemini = NewGeminiAdapter()
		}
		return gemini.Invoke(ctx, config.BaseURL, config.Model, apiKey, request)
	case providers.KindOpenAICompatible, providers.KindOllama:
		openAI := adapter.OpenAI
		if openAI == nil {
			openAI = NewOpenAICompatibleAdapter()
		}
		return openAI.Invoke(ctx, config.BaseURL, config.Model, apiKey, request)
	default:
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorInvalidResponse, Message: "unsupported provider kind " + config.Kind, Retryable: false},
		}, nil
	}
}
