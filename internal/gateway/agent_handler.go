package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	"github.com/go-chi/chi/v5"
)

func agentListHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agents, err := projectconfig.ListAgents(options.ConfigPath)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, agents, nil)
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
		agent, err := projectconfig.GetAgent(options.ConfigPath, chi.URLParam(request, "agent_id"))
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		writeSuccess(response, requestID, agent, nil)
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
