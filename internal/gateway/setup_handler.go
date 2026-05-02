package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/go-chi/chi/v5"
)

type modelSetupRequest struct {
	Preset    string `json:"preset"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`
}

type modelSetupResponse struct {
	Model consoleModelProfile `json:"model"`
}

func modelSetupHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Providers == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "models_unavailable", "Model registry is not initialized.", "Restart Gateway.")
			return
		}
		var body modelSetupRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send model setup fields as JSON.")
			return
		}
		profile := modelProfileFromSetup(body)
		if err := services.Providers.Save(request.Context(), profile); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "model_setup_failed", err.Error(), "Check provider kind, base URL, model, and api_key_env.")
			return
		}
		writeSuccess(response, requestID, modelSetupResponse{
			Model: sanitizeModelProfiles([]*providers.Profile{profile}, services.Secrets)[0],
		}, nil)
	}
}

func modelProfileFromSetup(body modelSetupRequest) *providers.Profile {
	switch strings.TrimSpace(strings.ToLower(body.Preset)) {
	case "deepseek_v4", "deepseek-v4", "deepseek":
		if body.ID == "" {
			body.ID = "deepseek_v4"
		}
		if body.Name == "" {
			body.Name = body.ID
		}
		if body.Kind == "" {
			body.Kind = providers.KindOpenAICompatible
		}
		if body.BaseURL == "" {
			body.BaseURL = "https://api.deepseek.com"
		}
		if body.Model == "" {
			body.Model = "deepseek-v4-pro"
		}
		if body.APIKeyEnv == "" {
			body.APIKeyEnv = "DEEPSEEK_API_KEY"
		}
	case "openai", "gpt":
		if body.ID == "" {
			body.ID = "gpt"
		}
		if body.Name == "" {
			body.Name = body.ID
		}
		if body.Kind == "" {
			body.Kind = providers.KindOpenAICompatible
		}
		if body.BaseURL == "" {
			body.BaseURL = "https://api.openai.com/v1"
		}
		if body.Model == "" {
			body.Model = "gpt-5.5"
		}
		if body.APIKeyEnv == "" {
			body.APIKeyEnv = "OPENAI_API_KEY"
		}
	}
	if body.Name == "" {
		body.Name = body.ID
	}
	return &providers.Profile{
		ID:        strings.TrimSpace(body.ID),
		Name:      strings.TrimSpace(body.Name),
		Kind:      providers.NormalizeKind(body.Kind),
		BaseURL:   strings.TrimRight(strings.TrimSpace(body.BaseURL), "/"),
		Model:     strings.TrimSpace(body.Model),
		APIKeyEnv: strings.TrimSpace(body.APIKeyEnv),
	}
}

type packInstallRequest struct {
	ModelID string `json:"model_id"`
	Force   bool   `json:"force"`
}

type packInstallResponse struct {
	PackID   string          `json:"pack_id"`
	ModelID  string          `json:"model_id"`
	Config   string          `json:"config_path"`
	Snapshot *graph.Snapshot `json:"graph_snapshot"`
}

func packInstallHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		packID := chi.URLParam(request, "packID")
		if packID != packs.DeveloperTeamID {
			writeError(response, http.StatusNotFound, requestID, "pack_not_found", "Bundled pack was not found.", "Run `nomici pack list`.")
			return
		}
		var body packInstallRequest
		if request.Body != nil {
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send model_id and force as JSON.")
				return
			}
		}
		result, err := packs.InstallDeveloperTeam(request.Context(), packs.InstallOptions{
			ConfigPath: options.ConfigPath,
			DBPath:     options.DBPath,
			ModelID:    strings.TrimSpace(body.ModelID),
			Force:      body.Force,
		})
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "pack_install_failed", err.Error(), "Configure a model profile first, then install the pack.")
			return
		}
		snapshot, err := compileAndSaveGraph(request.Context(), services.Graph, result.ConfigPath)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "graph_compile_failed", err.Error(), "Fix nomici.yaml and retry pack install.")
			return
		}
		writeSuccess(response, requestID, packInstallResponse{
			PackID:   result.PackID,
			ModelID:  result.ModelID,
			Config:   result.ConfigPath,
			Snapshot: snapshot,
		}, nil)
	}
}

type agentRunRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

type agentRunResponse struct {
	RunID           string              `json:"run_id"`
	AgentID         string              `json:"agent_id"`
	Status          string              `json:"status"`
	Messages        []adapters.Message  `json:"messages"`
	Usage           *adapters.UsageInfo `json:"usage,omitempty"`
	TraceEventCount int                 `json:"trace_event_count"`
}

func agentRunHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Graph == nil || services.Providers == nil || services.Secrets == nil || services.Adapter == nil || services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "run_unavailable", "Run services are not initialized.", "Restart Gateway.")
			return
		}
		var body agentRunRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send agent_id and prompt as JSON.")
			return
		}
		body.AgentID = strings.TrimSpace(body.AgentID)
		body.Prompt = strings.TrimSpace(body.Prompt)
		if body.AgentID == "" {
			body.AgentID = "product_pm"
		}
		if body.Prompt == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "prompt is required.", "Enter a task for the agent.")
			return
		}
		snapshot, err := services.Graph.Latest(request.Context())
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "graph_not_found", "No compiled graph snapshot was found.", "Install a pack or run graph validate.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "graph_load_failed", "Latest graph snapshot could not be loaded.", "Check Gateway logs.")
			return
		}
		agent, ok := snapshot.IR.Agents[body.AgentID]
		if !ok {
			writeError(response, http.StatusNotFound, requestID, "agent_not_found", "Agent was not found in the latest graph.", "Install Developer Team or choose an existing agent.")
			return
		}
		if agent.Kind != agentspec.AgentKindGateway && agent.Kind != agentspec.AgentKindModel {
			writeError(response, http.StatusBadRequest, requestID, "agent_not_runnable", "Console can currently run gateway_agent and model_agent nodes only.", "Use CLI for cli_agent-backed external agents.")
			return
		}
		model, ok := snapshot.IR.Models[agent.Model]
		if !ok {
			writeError(response, http.StatusBadRequest, requestID, "model_not_found", "Agent references a model missing from the graph.", "Reinstall the pack or validate nomici.yaml.")
			return
		}
		profile := graphModelToProvider(model)
		if stored, err := services.Providers.Get(request.Context(), model.ID); err == nil {
			profile = stored
		}
		result, err := invokeAgentModel(request.Context(), services, requestID, agent, profile, body.Prompt)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "run_failed", err.Error(), "Check the model provider and Gateway environment.")
			return
		}
		events, err := services.Trace.ListByRun(request.Context(), result.RunID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "trace_failed", "Run trace could not be loaded.", "Check Gateway logs.")
			return
		}
		result.TraceEventCount = len(events)
		writeSuccess(response, requestID, result, nil)
	}
}

func compileAndSaveGraph(ctx context.Context, graphStore *graph.Store, configPath string) (*graph.Snapshot, error) {
	if graphStore == nil {
		return nil, fmt.Errorf("graph store is not initialized")
	}
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, err
	}
	snapshot, validationErrors := graph.Compile(loaded)
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("AgentGraph validation failed with %d error(s)", len(validationErrors))
	}
	if err := graphStore.Save(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func graphModelToProvider(model graph.Model) *providers.Profile {
	capabilities := map[string]string{}
	for _, capability := range model.Capabilities {
		capabilities[capability] = "true"
	}
	return &providers.Profile{
		ID:            model.ID,
		Name:          model.ID,
		Kind:          model.Kind,
		BaseURL:       model.BaseURL,
		Model:         model.Model,
		APIKeyEnv:     model.APIKeyEnv,
		Capabilities:  capabilities,
		ContextWindow: model.ContextWindow,
	}
}

func agentPrompt(agent graph.Agent, prompt string) string {
	parts := []string{}
	if agent.Role != "" {
		parts = append(parts, "Role:\n"+agent.Role)
	}
	if agent.Instructions != "" {
		parts = append(parts, "Instructions:\n"+agent.Instructions)
	}
	parts = append(parts, "Task:\n"+prompt)
	return strings.Join(parts, "\n\n")
}

func invokeAgentModel(ctx context.Context, services Services, requestID string, agent graph.Agent, profile *providers.Profile, prompt string) (*agentRunResponse, error) {
	var apiKey string
	var redactions []string
	if profile.APIKeyEnv != "" {
		resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
		if !ok {
			return nil, fmt.Errorf("missing_secret: Gateway cannot see %s; restart Gateway from a shell where it is exported", profile.APIKeyEnv)
		}
		apiKey = resolved
		redactions = append(redactions, secrets.RedactedEnv(profile.APIKeyEnv))
	}
	runID := ids.New("run")
	if err := services.Trace.Append(ctx, &trace.Event{
		RunID:    runID,
		Type:     trace.EventRunStarted,
		NodeID:   agent.ID,
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "console"}),
	}); err != nil {
		return nil, fmt.Errorf("create run trace: %w", err)
	}
	if err := services.Trace.Append(ctx, &trace.Event{
		RunID:  runID,
		Type:   trace.EventModelRequested,
		NodeID: agent.ID,
		Payload: mustJSON(map[string]any{
			"provider_id":    profile.ID,
			"model":          profile.Model,
			"base_url":       profile.BaseURL,
			"prompt":         prompt,
			"api_key_source": profile.APIKeyEnv,
		}),
		Redactions: redactions,
		Metadata:   mustJSON(map[string]string{"request_id": requestID, "surface": "console"}),
	}); err != nil {
		return nil, fmt.Errorf("write model trace: %w", err)
	}
	result, err := services.Adapter.Invoke(ctx, profile.BaseURL, profile.Model, apiKey, adapters.InvokeRequest{
		RunID: runID,
		Messages: []adapters.Message{
			{Role: "user", Content: agentPrompt(agent, prompt)},
		},
	})
	if err != nil {
		_ = appendTraceFailure(ctx, services.Trace, runID, requestID, "adapter_failed", err.Error())
		return nil, fmt.Errorf("adapter failed: %w", err)
	}
	if result.Status != adapters.StatusCompleted {
		message := "provider invocation failed"
		if result.Error != nil {
			message = result.Error.Message
		}
		_ = appendTraceFailure(ctx, services.Trace, runID, requestID, "adapter_failed", message)
		return nil, fmt.Errorf("%s", message)
	}
	if err := services.Trace.Append(ctx, &trace.Event{
		RunID:  runID,
		Type:   trace.EventModelCompleted,
		NodeID: agent.ID,
		Payload: mustJSON(map[string]any{
			"provider_id": profile.ID,
			"model":       profile.Model,
			"usage":       result.Usage,
		}),
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "console"}),
	}); err != nil {
		return nil, fmt.Errorf("write completion trace: %w", err)
	}
	if err := services.Trace.Append(ctx, &trace.Event{
		RunID:    runID,
		Type:     trace.EventRunCompleted,
		NodeID:   agent.ID,
		Metadata: mustJSON(map[string]string{"request_id": requestID, "surface": "console"}),
	}); err != nil {
		return nil, fmt.Errorf("write run completion trace: %w", err)
	}
	return &agentRunResponse{
		RunID:    runID,
		AgentID:  agent.ID,
		Status:   result.Status,
		Messages: result.Messages,
		Usage:    result.Usage,
	}, nil
}
