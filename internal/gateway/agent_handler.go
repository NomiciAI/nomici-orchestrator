package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	"github.com/go-chi/chi/v5"
)

func agentListHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agents, err := projectconfig.ListAgents(options.ConfigPath)
		if err != nil {
			defaults := defaultTeamAgentRecords(request, services)
			if len(defaults) > 0 && (errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file")) {
				writeSuccess(response, requestID, defaults, []string{"Using the built-in default team until this project saves custom agents."})
				return
			}
			writeProjectConfigError(response, requestID, err)
			return
		}
		if len(agents) == 0 {
			if defaults := defaultTeamAgentRecords(request, services); len(defaults) > 0 {
				writeSuccess(response, requestID, defaults, []string{"Using the built-in default team until this project saves custom agents."})
				return
			}
		}
		writeSuccess(response, requestID, agents, nil)
	}
}

func defaultTeamAgentRecords(request *http.Request, services Services) []projectconfig.AgentRecord {
	if services.Providers == nil {
		return nil
	}
	profiles, err := services.Providers.List(request.Context())
	if err != nil || len(profiles) == 0 {
		return nil
	}
	modelID := profiles[0].ID
	agents := defaultTeamAgents(modelID, agentspec.Source{File: "builtin", Path: "default_team"})
	order := []string{"product_pm", "planner", "researcher", "coder", "reporter"}
	records := make([]projectconfig.AgentRecord, 0, len(order))
	for _, id := range order {
		agent, ok := agents[id]
		if !ok {
			continue
		}
		records = append(records, projectconfig.AgentRecord{
			ID:           agent.ID,
			Name:         defaultTeamAgentName(agent.ID),
			Description:  "Built-in default team role. Save a copy to customize it for this project.",
			Source:       "built_in",
			Kind:         agent.Kind,
			Model:        agent.Model,
			Runtime:      agent.Runtime,
			Role:         agent.Role,
			Instructions: agent.Instructions,
			Tools:        agent.Tools,
			Skills:       agent.Skills,
			Tags:         []string{"default-team"},
			Capabilities: agent.Capabilities,
		})
	}
	return records
}

func resolveAgentRecord(request *http.Request, options Options, services Services, agentID string) (*projectconfig.AgentRecord, bool, error) {
	agent, err := projectconfig.GetAgent(options.ConfigPath, agentID)
	if err == nil {
		return agent, false, nil
	}
	for _, record := range defaultTeamAgentRecords(request, services) {
		if record.ID == agentID {
			copy := record
			return &copy, true, nil
		}
	}
	return nil, false, err
}

func defaultTeamAgentName(id string) string {
	switch id {
	case "product_pm":
		return "Product PM"
	case "planner":
		return "Planner"
	case "researcher":
		return "Researcher"
	case "coder":
		return "Coder"
	case "reporter":
		return "Reporter"
	default:
		return id
	}
}

func agentCreateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body projectconfig.AgentRecord
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send an agent definition.")
			return
		}
		if _, err := projectconfig.UpsertAgent(request.Context(), options.ConfigPath, options.DBPath, body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_save_failed", err.Error(), "Fix the agent fields and retry.")
			return
		}
		agent, err := projectconfig.GetAgent(options.ConfigPath, body.ID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, agent, nil)
	}
}

func agentDetailHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agent, _, err := resolveAgentRecord(request, options, services, chi.URLParam(request, "agent_id"))
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, agent, nil)
	}
}

func agentCopyHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agentID := chi.URLParam(request, "agent_id")
		agent, builtIn, err := resolveAgentRecord(request, options, services, agentID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		if !builtIn {
			writeError(response, http.StatusConflict, requestID, "agent_already_project_owned", "Agent is already saved in this project.", "Edit the project agent directly.")
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		if strings.TrimSpace(body.ID) != "" {
			agent.ID = strings.TrimSpace(body.ID)
		}
		agent.Source = ""
		if strings.TrimSpace(agent.Model) != "" {
			if err := projectconfig.EnsureModelReference(options.ConfigPath, agent.Model); err != nil {
				writeError(response, http.StatusBadRequest, requestID, "agent_copy_failed", err.Error(), "Fix the model profile and retry.")
				return
			}
		}
		if _, err := projectconfig.UpsertAgent(request.Context(), options.ConfigPath, options.DBPath, *agent); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_copy_failed", err.Error(), "Fix the target agent id and retry.")
			return
		}
		copied, err := projectconfig.GetAgent(options.ConfigPath, agent.ID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, copied, []string{fmt.Sprintf("Copied %s into project config.", agentID)})
	}
}

func agentUpdateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agentID := chi.URLParam(request, "agent_id")
		existing, err := projectconfig.GetAgent(options.ConfigPath, agentID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		var body projectconfig.AgentRecord
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send fields to update.")
			return
		}
		merged := mergeAgentRecord(*existing, body)
		merged.ID = agentID
		if _, err := projectconfig.UpsertAgent(request.Context(), options.ConfigPath, options.DBPath, merged); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_save_failed", err.Error(), "Fix the agent fields and retry.")
			return
		}
		agent, err := projectconfig.GetAgent(options.ConfigPath, agentID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, agent, nil)
	}
}

func agentDeleteHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if _, err := projectconfig.DeleteAgent(request.Context(), options.ConfigPath, options.DBPath, chi.URLParam(request, "agent_id")); err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, map[string]string{"status": "deleted"}, nil)
	}
}

func agentValidateHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body projectconfig.AgentRecord
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send an agent definition.")
			return
		}
		if body.ID == "" {
			body.ID = chi.URLParam(request, "agent_id")
		}
		if err := projectconfig.ValidateAgent(body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_invalid", err.Error(), "Fix the agent fields and retry.")
			return
		}
		writeSuccess(response, requestID, map[string]string{"status": "valid"}, nil)
	}
}

func orchestrationShowHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		config, err := projectconfig.GetOrchestration(options.ConfigPath)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, config, nil)
	}
}

func orchestrationUpdateHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body projectconfig.OrchestrationConfig
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send orchestration settings.")
			return
		}
		if _, err := projectconfig.SaveOrchestration(request.Context(), options.ConfigPath, options.DBPath, body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "orchestration_save_failed", err.Error(), "Fix the role flow and retry.")
			return
		}
		config, err := projectconfig.GetOrchestration(options.ConfigPath)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, config, nil)
	}
}

func orchestrationValidateHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body projectconfig.OrchestrationConfig
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send orchestration settings.")
			return
		}
		writeSuccess(response, requestID, map[string]any{
			"status": "valid",
			"config": body,
		}, nil)
	}
}

func mergeAgentRecord(existing projectconfig.AgentRecord, update projectconfig.AgentRecord) projectconfig.AgentRecord {
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Kind != "" {
		existing.Kind = update.Kind
	}
	if update.Model != "" {
		existing.Model = update.Model
	}
	if update.Runtime != "" {
		existing.Runtime = update.Runtime
	}
	if update.Role != "" {
		existing.Role = update.Role
	}
	if update.Instructions != "" {
		existing.Instructions = update.Instructions
	}
	if update.Tools != nil {
		existing.Tools = update.Tools
	}
	if update.Skills != nil {
		existing.Skills = update.Skills
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}
	if update.Triggers != nil {
		existing.Triggers = update.Triggers
	}
	if update.Capabilities != nil {
		existing.Capabilities = update.Capabilities
	}
	if update.Permissions != nil {
		existing.Permissions = update.Permissions
	}
	if update.RuntimeProfile != nil {
		existing.RuntimeProfile = update.RuntimeProfile
	}
	if update.ApprovalPolicy != "" {
		existing.ApprovalPolicy = update.ApprovalPolicy
	}
	return existing
}

func writeProjectConfigError(response http.ResponseWriter, requestID string, err error) {
	if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file") {
		writeError(response, http.StatusNotFound, requestID, "config_not_found", "Project config was not found.", "Run `nomici setup` first.")
		return
	}
	if strings.Contains(err.Error(), "was not found") {
		writeError(response, http.StatusNotFound, requestID, "agent_not_found", err.Error(), "Refresh agents.")
		return
	}
	writeError(response, http.StatusBadRequest, requestID, "project_config_failed", err.Error(), "Fix nomici.yaml and retry.")
}
