package gateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/go-chi/chi/v5"
)

func toolListHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), toolbroker.Definitions(), nil)
	}
}

func toolDetailHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		definition, err := toolbroker.DefinitionByID(chi.URLParam(request, "tool_id"))
		if err != nil {
			writeError(response, http.StatusNotFound, requestID, "tool_not_found", "Tool was not found.", "Refresh tool registry.")
			return
		}
		writeSuccess(response, requestID, definition, nil)
	}
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
		ConfigPath: options.ConfigPath,
	}
}
