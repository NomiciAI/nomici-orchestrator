package gateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/chats"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/go-chi/chi/v5"
)

type chatCreateRequest struct {
	Title          string   `json:"title"`
	AgentID        string   `json:"agent_id"`
	Prompt         string   `json:"prompt"`
	SelectedSkills []string `json:"selected_skills"`
}

type chatMessageRequest struct {
	AgentID        string   `json:"agent_id"`
	Content        string   `json:"content"`
	SelectedSkills []string `json:"selected_skills"`
}

type chatMessageResponse struct {
	Message          *chats.Message               `json:"message"`
	AssistantMessage *chats.Message               `json:"assistant_message,omitempty"`
	Run              *runCreateResponse           `json:"run,omitempty"`
	RouteDecision    *orchestration.RouteDecision `json:"route_decision,omitempty"`
	Clarification    string                       `json:"clarification,omitempty"`
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
		messageResponse, startErr := addChatMessageAndMaybeRun(request, options, services, thread.ChatID, body.AgentID, body.Prompt, body.SelectedSkills)
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
		result, startErr := addChatMessageAndMaybeRun(request, options, services, chi.URLParam(request, "chat_id"), body.AgentID, body.Content, body.SelectedSkills)
		if startErr != nil {
			writeError(response, startErr.Status, requestID, startErr.Code, startErr.Message, startErr.Remediation)
			return
		}
		writeSuccess(response, requestID, result, nil)
	}
}

func addChatMessageAndMaybeRun(request *http.Request, options Options, services Services, chatID string, agentID string, content string, selectedSkills []string) (*chatMessageResponse, *startRunError) {
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
	manualAgentID := strings.TrimSpace(agentID)
	if strings.EqualFold(manualAgentID, "auto") {
		manualAgentID = ""
	}
	var snapshotRoute *orchestration.RouteDecision
	conversation := recentChatContext(request, services, chatID, 8)
	if services.Graph != nil {
		if snapshot, err := services.Graph.Latest(request.Context()); err == nil {
			decision := routeChatIntent(request.Context(), services, content, manualAgentID, snapshot, conversation)
			snapshotRoute = &decision
		}
	}
	if snapshotRoute == nil {
		decision := orchestration.Route(content, manualAgentID, nil)
		snapshotRoute = &decision
	}
	if len(selectedSkills) > 0 {
		snapshotRoute.RequiredSkills = uniqueStrings(append(snapshotRoute.RequiredSkills, selectedSkills...))
		snapshotRoute.Rationale = strings.TrimSpace(snapshotRoute.Rationale + " User-selected skills were added to the run context.")
	}
	if snapshotRoute.Mode != orchestration.ModeWorkspaceRun {
		reply := orchestration.DirectReply(*snapshotRoute)
		assistantMessage, err := services.Chats.AddMessage(request.Context(), &chats.Message{
			ChatID:   chatID,
			Role:     chats.RoleAssistant,
			Content:  reply,
			Metadata: routeMetadata(snapshotRoute),
		})
		if err != nil {
			return nil, &startRunError{Status: http.StatusInternalServerError, Code: "chat_message_failed", Message: "Chat response could not be saved.", Remediation: "Check Gateway logs."}
		}
		return &chatMessageResponse{Message: message, AssistantMessage: assistantMessage, RouteDecision: snapshotRoute, Clarification: snapshotRoute.Clarification}, nil
	}
	started, startErr := startWorkspaceRunWithRoute(request.Context(), options, services, manualAgentID, content, "chat", map[string]any{"chat_id": chatID, "message_id": message.MessageID}, snapshotRoute, false)
	if startErr != nil {
		return nil, startErr
	}
	if err := services.Chats.UpdateMessageRun(request.Context(), message.MessageID, started.Response.RunID, started.Response.SessionID); err == nil {
		message.RunID = started.Response.RunID
		message.SessionID = started.Response.SessionID
	}
	startRunWorker(options, services, started.Executor, started.Request, started.Session, started.Tasks)
	return &chatMessageResponse{Message: message, Run: &started.Response, RouteDecision: snapshotRoute}, nil
}

func recentChatContext(request *http.Request, services Services, chatID string, limit int) string {
	if services.Chats == nil || limit <= 0 {
		return ""
	}
	detail, err := services.Chats.Detail(request.Context(), chatID)
	if err != nil {
		return ""
	}
	messages := detail.Messages
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		content := strings.Join(strings.Fields(message.Content), " ")
		if content == "" {
			continue
		}
		if len(content) > 240 {
			content = content[:237] + "..."
		}
		lines = append(lines, message.Role+": "+content)
	}
	return strings.Join(lines, "\n")
}

func routeMetadata(route *orchestration.RouteDecision) json.RawMessage {
	if route == nil {
		return json.RawMessage("{}")
	}
	payload, err := json.Marshal(map[string]any{"route_decision": route})
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func trimTitle(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 72 {
		return value
	}
	return value[:69] + "..."
}
