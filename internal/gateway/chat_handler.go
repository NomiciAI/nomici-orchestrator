package gateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/chats"
	"github.com/go-chi/chi/v5"
)

type chatCreateRequest struct {
	Title   string `json:"title"`
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

type chatMessageRequest struct {
	AgentID string `json:"agent_id"`
	Content string `json:"content"`
}

type chatMessageResponse struct {
	Message *chats.Message     `json:"message"`
	Run     *runCreateResponse `json:"run,omitempty"`
}

func chatListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Chats == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "chats_unavailable", "Chat store is not initialized.", "Restart Gateway.")
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
		threads, err := services.Chats.ListThreads(request.Context(), limit)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "chats_list_failed", "Chats could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, threads, nil)
	}
}

func chatCreateHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Chats == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "chats_unavailable", "Chat store is not initialized.", "Restart Gateway.")
			return
		}
		var body chatCreateRequest
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		title := strings.TrimSpace(body.Title)
		if title == "" && strings.TrimSpace(body.Prompt) != "" {
			title = trimTitle(body.Prompt)
		}
		thread, err := services.Chats.CreateThread(request.Context(), title, nil)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "chat_create_failed", "Chat could not be created.", "Check Gateway logs.")
			return
		}
		if strings.TrimSpace(body.Prompt) == "" {
			detail, _ := services.Chats.Detail(request.Context(), thread.ChatID)
			writeSuccess(response, requestID, detail, nil)
			return
		}
		messageResponse, startErr := addChatMessageAndMaybeRun(request, options, services, thread.ChatID, body.AgentID, body.Prompt)
		if startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		writeSuccess(response, requestID, messageResponse, nil)
	}
}

func chatDetailHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Chats == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "chats_unavailable", "Chat store is not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Chats.Detail(request.Context(), chi.URLParam(request, "chat_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
				writeError(response, http.StatusNotFound, requestID, "chat_not_found", "Chat was not found.", "Refresh chats.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "chat_load_failed", "Chat could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, detail, nil)
	}
}

func chatMessageHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body chatMessageRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send content and optional agent_id as JSON.")
			return
		}
		result, startErr := addChatMessageAndMaybeRun(request, options, services, chi.URLParam(request, "chat_id"), body.AgentID, body.Content)
		if startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		writeSuccess(response, requestID, result, nil)
	}
}

func addChatMessageAndMaybeRun(request *http.Request, options Options, services Services, chatID string, agentID string, content string) (*chatMessageResponse, *startRunError) {
	if services.Chats == nil {
		return nil, &startRunError{Status: http.StatusServiceUnavailable, Code: "chats_unavailable", Message: "Chat store is not initialized.", Remediation: "Restart Gateway."}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, &startRunError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "content is required.", Remediation: "Send a non-empty chat message."}
	}
	if _, err := services.Chats.GetThread(request.Context(), chatID); err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return nil, &startRunError{Status: http.StatusNotFound, Code: "chat_not_found", Message: "Chat was not found.", Remediation: "Refresh chats."}
		}
		return nil, &startRunError{Status: http.StatusInternalServerError, Code: "chat_load_failed", Message: "Chat could not be loaded.", Remediation: "Check Gateway logs."}
	}
	message, err := services.Chats.AddMessage(request.Context(), &chats.Message{
		ChatID:  chatID,
		Role:    chats.RoleUser,
		Content: content,
	})
	if err != nil {
		return nil, &startRunError{Status: http.StatusInternalServerError, Code: "chat_message_failed", Message: "Chat message could not be saved.", Remediation: "Check Gateway logs."}
	}
	started, startErr := startWorkspaceRun(request.Context(), options, services, agentID, content, "chat", map[string]any{"chat_id": chatID, "message_id": message.MessageID})
	if startErr != nil {
		return nil, startErr
	}
	if err := services.Chats.UpdateMessageRun(request.Context(), message.MessageID, started.Response.RunID, started.Response.SessionID); err == nil {
		message.RunID = started.Response.RunID
		message.SessionID = started.Response.SessionID
	}
	return &chatMessageResponse{Message: message, Run: &started.Response}, nil
}

func trimTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 72 {
		return value
	}
	return value[:69] + "..."
}
