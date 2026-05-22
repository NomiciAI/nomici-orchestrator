package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	"github.com/NomiciAI/nomici-orchestrator/internal/skills"
	"github.com/go-chi/chi/v5"
)

type skillWriteRequest struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Triggers      []string `json:"triggers"`
	Files         []string `json:"files"`
	RequiredTools []string `json:"required_tools"`
	Risk          string   `json:"risk"`
	Compatibility string   `json:"compatibility"`
	Briefing      string   `json:"briefing"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

type skillPatchRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type skillImportRequest struct {
	Path string `json:"path"`
}

func skillListHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), skills.List(options.ConfigPath), nil)
	}
}

func skillCreateHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body skillWriteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send skill metadata and briefing.")
			return
		}
		definition := skills.Definition{
			ID:            strings.TrimSpace(body.ID),
			Name:          strings.TrimSpace(body.Name),
			Description:   strings.TrimSpace(body.Description),
			Triggers:      body.Triggers,
			Files:         body.Files,
			RequiredTools: body.RequiredTools,
			Risk:          strings.TrimSpace(body.Risk),
			Compatibility: strings.TrimSpace(body.Compatibility),
			Briefing:      strings.TrimSpace(body.Briefing),
		}
		if body.Enabled != nil {
			definition.Disabled = !*body.Enabled
		}
		if err := projectconfig.UpsertSkill(options.ConfigPath, definition); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "skill_save_failed", err.Error(), "Fix the skill fields and retry.")
			return
		}
		saved, err := skills.Get(options.ConfigPath, definition.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "skill_load_failed", "Skill was saved but could not be reloaded.", "Refresh skills.")
			return
		}
		writeSuccess(response, requestID, saved, nil)
	}
}

func skillImportHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body skillImportRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send path to a local skill directory.")
			return
		}
		definition, err := skills.LoadDirectory(body.Path)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "skill_import_failed", err.Error(), "Choose a local directory with skill metadata or briefing content.")
			return
		}
		if err := projectconfig.UpsertSkill(options.ConfigPath, definition); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "skill_save_failed", err.Error(), "Fix the imported skill metadata and retry.")
			return
		}
		saved, err := skills.Get(options.ConfigPath, definition.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "skill_load_failed", "Skill was imported but could not be reloaded.", "Refresh skills.")
			return
		}
		writeSuccess(response, requestID, saved, nil)
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

func skillPatchHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body skillPatchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send enabled true or false.")
			return
		}
		if body.Enabled == nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "enabled is required.", "Send enabled true or false.")
			return
		}
		id := chi.URLParam(request, "skill_id")
		if err := projectconfig.SetSkillEnabled(options.ConfigPath, id, *body.Enabled); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "skill_update_failed", err.Error(), "Refresh skill registry and retry.")
			return
		}
		definition, err := skills.Get(options.ConfigPath, id)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "skill_load_failed", "Skill was updated but could not be reloaded.", "Refresh skills.")
			return
		}
		writeSuccess(response, requestID, definition, nil)
	}
}

func skillDeleteHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		id := chi.URLParam(request, "skill_id")
		if err := projectconfig.DeleteSkill(options.ConfigPath, id); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "skill_delete_failed", err.Error(), "Only project skills can be deleted; disable built-in skills instead.")
			return
		}
		writeSuccess(response, requestID, map[string]string{"status": "deleted"}, nil)
	}
}
