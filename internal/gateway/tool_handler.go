package gateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/go-chi/chi/v5"
)

func toolListHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), toolDefinitionsWithStatus(options, services), nil)
	}
}

func toolDetailHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		definition, err := toolbroker.DefinitionByID(chi.URLParam(request, "tool_id"))
		if err != nil {
			writeError(response, http.StatusNotFound, requestID, "tool_not_found", "Tool was not found.", "Refresh tool registry.")
			return
		}
		definition.ExecutionStatus = toolExecutionStatus(options, services, definition.ID)
		writeSuccess(response, requestID, definition, nil)
	}
}

func toolDefinitionsWithStatus(options Options, services Services) []toolbroker.Definition {
	definitions := toolbroker.Definitions()
	for index := range definitions {
		definitions[index].ExecutionStatus = toolExecutionStatus(options, services, definitions[index].ID)
	}
	return definitions
}

func toolExecutionStatus(options Options, services Services, toolID string) string {
	switch toolID {
	case toolbroker.ToolSearch:
		provider := configuredToolProvider(options.ConfigPath, "web_search")
		switch provider {
		case "", "duckduckgo":
			return "executable"
		case "none":
			return "unavailable"
		default:
			return "configured_only"
		}
	case toolbroker.ToolFetch:
		provider := configuredToolProvider(options.ConfigPath, "web_fetch")
		switch provider {
		case "", "jina_reader", "direct_http":
			return "executable"
		case "none":
			return "unavailable"
		default:
			return "configured_only"
		}
	}
	intent, err := sandboxIntentFromConfig(options.ConfigPath)
	if err != nil || intent.Mode == "" || intent.Mode == "none" {
		return "unavailable"
	}
	if services.Sandboxes == nil {
		return "configured_only"
	}
	switch toolID {
	case toolbroker.ToolWriteFile, toolbroker.ToolReplaceFile, toolbroker.ToolPresentArtifact:
		if intent.FileWriteEnabled {
			return "executable"
		}
		return "configured_only"
	case toolbroker.ToolBash:
		if intent.BashEnabled {
			return "executable"
		}
		return "configured_only"
	default:
		return "executable"
	}
}

func configuredToolProvider(configPath string, toolID string) string {
	if configPath == "" {
		configPath = "nomici.yaml"
	}
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil || loaded.Spec == nil || loaded.Spec.Tools == nil {
		return ""
	}
	config := loaded.Spec.Tools[toolID]
	provider, _ := config["provider"].(string)
	if provider == "" {
		provider, _ = config["kind"].(string)
	}
	return strings.TrimSpace(strings.ToLower(strings.ReplaceAll(provider, "-", "_")))
}

func sessionToolCallsHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Tools == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "tools_unavailable", "Tool call store is not initialized.", "Restart Gateway.")
			return
		}
		records, err := services.Tools.ListBySession(request.Context(), chi.URLParam(request, "session_id"), 50)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "tool_calls_failed", "Tool calls could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, records, nil)
	}
}

func sessionToolCallCreateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Tools == nil || services.Policy == nil || services.Runs == nil || services.Sandboxes == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "tools_unavailable", "Tool execution services are not initialized.", "Restart Gateway.")
			return
		}
		sessionID := chi.URLParam(request, "session_id")
		if _, err := services.Runs.GetBySession(request.Context(), sessionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "session_load_failed", "Run session could not be loaded.", "Check Gateway logs.")
			return
		}
		var body toolbroker.ExecuteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send tool_id and input as JSON.")
			return
		}
		body.SessionID = sessionID
		broker := newToolBroker(options, services)
		record, err := broker.Execute(request.Context(), body)
		if err != nil && record == nil {
			writeError(response, http.StatusBadRequest, requestID, "tool_call_failed", err.Error(), "Check the tool input and workspace policy.")
			return
		}
		writeSuccess(response, requestID, record, nil)
	}
}

func newToolBroker(options Options, services Services) *toolbroker.Broker {
	return &toolbroker.Broker{
		Store:      services.Tools,
		Policy:     services.Policy,
		Trace:      services.Trace,
		Runs:       services.Runs,
		Sandboxes:  services.Sandboxes,
		Artifacts:  services.Artifacts,
		Locks:      services.Locks,
		ConfigPath: options.ConfigPath,
	}
}
