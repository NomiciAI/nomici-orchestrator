package gateway

import (
	"database/sql"
	"errors"
	"net/http"

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
