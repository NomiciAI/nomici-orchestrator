package adapters

import (
	"context"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

type ModelAdapter struct {
	OpenAI *OpenAICompatibleAdapter
	Codex  *CodexCLIAdapter
}

func NewModelAdapter() *ModelAdapter {
	return &ModelAdapter{
		OpenAI: NewOpenAICompatibleAdapter(),
		Codex:  NewCodexCLIAdapter(),
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
