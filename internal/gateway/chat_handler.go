package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/chats"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
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
		reply := chatDirectReply(request, services, content, snapshotRoute)
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
		if startErr.Code == "run_not_supported" && strings.Contains(strings.ToLower(startErr.Message), "agent_id is required") {
			reply := "I can chat here, but workspace runs need at least one configured agent. Open Settings > Agent Builder to create an agent, or run `nomici setup --pack developer-team` from the project root."
			assistantMessage, err := services.Chats.AddMessage(request.Context(), &chats.Message{
				ChatID:   chatID,
				Role:     chats.RoleAssistant,
				Content:  reply,
				Metadata: routeMetadata(snapshotRoute),
			})
			if err != nil {
				return nil, &startRunError{Status: http.StatusInternalServerError, Code: "chat_message_failed", Message: "Chat response could not be saved.", Remediation: "Check Gateway logs."}
			}
			return &chatMessageResponse{Message: message, AssistantMessage: assistantMessage, RouteDecision: snapshotRoute}, nil
		}
		return nil, startErr
	}
	if err := services.Chats.UpdateMessageRun(request.Context(), message.MessageID, started.Response.RunID, started.Response.SessionID); err == nil {
		message.RunID = started.Response.RunID
		message.SessionID = started.Response.SessionID
	}
	startRunWorker(options, services, started.Executor, started.Request, started.Session, started.Tasks)
	return &chatMessageResponse{Message: message, Run: &started.Response, RouteDecision: snapshotRoute}, nil
}

func chatDirectReply(request *http.Request, services Services, content string, route *orchestration.RouteDecision) string {
	fallback := ""
	if route != nil {
		fallback = orchestration.DirectReply(*route)
	}
	if fallback == "" {
		fallback = "I can help here in chat. Ask a normal question, or describe a larger goal and I’ll open a workspace when it needs planning, files, tools, or agents."
	}
	if route == nil || route.Mode != orchestration.ModeDirectReply || services.Adapter == nil || services.Secrets == nil {
		return fallback
	}
	profile, err := directReplyProfile(request, services, route)
	if err != nil || profile == nil {
		return fallback
	}
	apiKey := ""
	if profile.APIKeyEnv != "" {
		resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
		if !ok {
			return fallback
		}
		apiKey = resolved
	}
	ctx, cancel := contextWithTimeout(request, 30*time.Second)
	defer cancel()
	result, err := services.Adapter.Invoke(ctx, adapters.ModelConfig{
		Kind:    profile.Kind,
		BaseURL: profile.BaseURL,
		Model:   profile.Model,
	}, apiKey, adapters.InvokeRequest{
		NodeID: "chat_direct_reply",
		Messages: []adapters.Message{
			{Role: "system", Content: "You are Nomici, a concise local-first multi-agent workspace assistant. Answer directly in chat. Do not ask the user to provide target outcomes unless they are explicitly asking Nomici to run a larger task."},
			{Role: "user", Content: content},
		},
		Options: adapters.InvokeOptions{TimeoutMs: 30000},
	})
	if err != nil || result == nil || result.Status != adapters.StatusCompleted || len(result.Messages) == 0 {
		return fallback
	}
	reply := strings.TrimSpace(result.Messages[len(result.Messages)-1].Content)
	if reply == "" {
		return fallback
	}
	return reply
}

func directReplyProfile(request *http.Request, services Services, route *orchestration.RouteDecision) (*providers.Profile, error) {
	if services.Graph != nil {
		if snapshot, err := services.Graph.Latest(request.Context()); err == nil && snapshot != nil {
			if route != nil && strings.TrimSpace(route.RecommendedAgentID) != "" {
				if agent, ok := snapshot.IR.Agents[strings.TrimSpace(route.RecommendedAgentID)]; ok && agent.Model != "" {
					if model, ok := snapshot.IR.Models[agent.Model]; ok {
						return graphModelToGatewayProvider(model), nil
					}
				}
			}
			for _, model := range snapshot.IR.Models {
				return graphModelToGatewayProvider(model), nil
			}
		}
	}
	if services.Providers == nil {
		return nil, fmt.Errorf("provider store unavailable")
	}
	profiles, err := services.Providers.List(request.Context())
	if err != nil || len(profiles) == 0 {
		return nil, err
	}
	return profiles[0], nil
}

func contextWithTimeout(request *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), timeout)
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
