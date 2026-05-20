package gateway

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/go-chi/chi/v5"
)

func memoryProposalListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Memory == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "memory_unavailable", "Memory proposal store is not initialized.", "Restart Gateway.")
			return
		}
		proposals, err := services.Memory.List(request.Context(), request.URL.Query().Get("status"), 50)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "memory_list_failed", "Memory proposals could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, proposals, nil)
	}
}

func memoryItemListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Context == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "memory_unavailable", "Shared context store is not initialized.", "Restart Gateway.")
			return
		}
		limit := 50
		if raw := request.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(response, http.StatusBadRequest, requestID, "invalid_request", "limit must be between 1 and 100.", "Use a positive integer limit.")
				return
			}
			limit = parsed
		}
		items, err := services.Context.ListItems(request.Context(), request.URL.Query().Get("project_id"), sharedcontext.ScopeProject, limit)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "memory_items_failed", "Reusable context could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, items, nil)
	}
}

func memoryItemDeleteHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Context == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "memory_unavailable", "Shared context store is not initialized.", "Restart Gateway.")
			return
		}
		if err := services.Context.SetItemStatus(request.Context(), chi.URLParam(request, "context_id"), sharedcontext.StatusDeleted); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "memory_item_delete_failed", err.Error(), "Refresh memory and retry.")
			return
		}
		writeSuccess(response, requestID, map[string]string{"status": sharedcontext.StatusDeleted}, nil)
	}
}

func memoryProposalApproveHandler(services Services) http.HandlerFunc {
	return memoryProposalTransitionHandler(services, "approve")
}

func memoryProposalRejectHandler(services Services) http.HandlerFunc {
	return memoryProposalTransitionHandler(services, "reject")
}

func memoryProposalDeleteHandler(services Services) http.HandlerFunc {
	return memoryProposalTransitionHandler(services, "delete")
}

func memoryProposalTransitionHandler(services Services, action string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Memory == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "memory_unavailable", "Memory proposal store is not initialized.", "Restart Gateway.")
			return
		}
		proposalID := chi.URLParam(request, "proposal_id")
		var proposal any
		var err error
		switch action {
		case "approve":
			proposal, err = services.Memory.Approve(request.Context(), proposalID, services.Context)
		case "reject":
			proposal, err = services.Memory.Reject(request.Context(), proposalID)
		case "delete":
			proposal, err = services.Memory.Delete(request.Context(), proposalID)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "memory_not_found", "Memory proposal was not found.", "Refresh memory proposals.")
				return
			}
			writeError(response, http.StatusBadRequest, requestID, "memory_update_failed", err.Error(), "Refresh memory proposals and retry.")
			return
		}
		writeSuccess(response, requestID, proposal, nil)
	}
}
