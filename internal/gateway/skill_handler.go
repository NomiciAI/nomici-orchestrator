package gateway

import (
	"net/http"

	"github.com/NomiciAI/nomici-orchestrator/internal/skills"
	"github.com/go-chi/chi/v5"
)

func skillListHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), skills.List(options.ConfigPath), nil)
	}
}

func skillDetailHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		definition, err := skills.Get(options.ConfigPath, chi.URLParam(request, "skill_id"))
		if err != nil {
			writeError(response, http.StatusNotFound, requestID, "skill_not_found", "Skill was not found.", "Refresh skill registry.")
			return
		}
		writeSuccess(response, requestID, definition, nil)
	}
}
