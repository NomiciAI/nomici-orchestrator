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

type chatFeedbackRequest struct {
	MessageID string         `json:"message_id"`
	Score     string         `json:"score"`
	Note      string         `json:"note"`
	Metadata  map[string]any `json:"metadata"`
}

type chatMessageResponse struct {
	Message          *chats.Message               `json:"message"`
	AssistantMessage *chats.Message               `json:"assistant_message,omitempty"`
	Run              *runCreateResponse           `json:"run,omitempty"`
	RouteDecision    *orchestration.RouteDecision `json:"route_decision,omitempty"`
	Clarification    string                       `json:"clarification,omitempty"`
	AssistantSource  string                       `json:"assistant_source,omitempty"`
	ModelProfileID   string                       `json:"model_profile_id,omitempty"`
	ProviderError    *chatProviderError           `json:"provider_error,omitempty"`
}

type chatProviderError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
	ProviderID  string `json:"provider_id,omitempty"`
	ModelID     string `json:"model_id,omitempty"`
}

type directChatResult struct {
	Content        string
	Source         string
	ModelProfileID string
	ProviderError  *chatProviderError
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

func chatSuggestionsHandler(services Services) http.HandlerFunc {
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
		writeSuccess(response, requestID, chatSuggestions(detail), nil)
	}
}

func chatFeedbackHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Chats == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "chats_unavailable", "Chat store is not initialized.", "Restart Gateway.")
			return
		}
		chatID := chi.URLParam(request, "chat_id")
		var body chatFeedbackRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "Send message_id, score, and optional note.")
			return
		}
		score := strings.ToLower(strings.TrimSpace(body.Score))
		if score != "up" && score != "down" && score != "neutral" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_feedback", "Feedback score must be up, down, or neutral.", "Choose a supported score.")
			return
		}
		messageID := strings.TrimSpace(body.MessageID)
		if messageID == "" {
			writeError(response, http.StatusBadRequest, requestID, "invalid_feedback", "message_id is required.", "Send feedback for a specific chat message.")
			return
		}
		if _, err := services.Chats.GetThread(request.Context(), chatID); err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
				writeError(response, http.StatusNotFound, requestID, "chat_not_found", "Chat was not found.", "Refresh chats.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "chat_load_failed", "Chat could not be loaded.", "Check Gateway logs.")
			return
		}
		metadata := json.RawMessage("{}")
		if body.Metadata != nil {
			payload, err := json.Marshal(body.Metadata)
			if err != nil {
				writeError(response, http.StatusBadRequest, requestID, "invalid_feedback", "metadata must be JSON serializable.", "Remove unsupported metadata values.")
				return
			}
			metadata = payload
		}
		feedback, err := services.Chats.UpsertFeedback(request.Context(), &chats.Feedback{
			ChatID:    chatID,
			MessageID: messageID,
			Score:     score,
			Note:      strings.TrimSpace(body.Note),
			Metadata:  metadata,
		})
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "feedback_save_failed", "Feedback could not be saved.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, feedback, nil)
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
		if reply.ProviderError != nil {
			return &chatMessageResponse{
				Message:         message,
				RouteDecision:   snapshotRoute,
				Clarification:   snapshotRoute.Clarification,
				AssistantSource: reply.Source,
				ModelProfileID:  reply.ModelProfileID,
				ProviderError:   reply.ProviderError,
			}, nil
		}
		assistantMessage, err := services.Chats.AddMessage(request.Context(), &chats.Message{
			ChatID:   chatID,
			Role:     chats.RoleAssistant,
			Content:  reply.Content,
			Metadata: routeMetadataWithSource(snapshotRoute, reply.Source, reply.ModelProfileID),
		})
		if err != nil {
			return nil, &startRunError{Status: http.StatusInternalServerError, Code: "chat_message_failed", Message: "Chat response could not be saved.", Remediation: "Check Gateway logs."}
		}
		return &chatMessageResponse{
			Message:          message,
			AssistantMessage: assistantMessage,
			RouteDecision:    snapshotRoute,
			Clarification:    snapshotRoute.Clarification,
			AssistantSource:  reply.Source,
			ModelProfileID:   reply.ModelProfileID,
		}, nil
	}
	started, startErr := startWorkspaceRunWithRoute(request.Context(), options, services, manualAgentID, content, "chat", map[string]any{"chat_id": chatID, "message_id": message.MessageID}, snapshotRoute, false)
	if startErr != nil {
		if startErr.Code == "model_not_configured" || (startErr.Code == "run_not_supported" && strings.Contains(strings.ToLower(startErr.Message), "agent_id is required")) {
			return &chatMessageResponse{
				Message:         message,
				RouteDecision:   snapshotRoute,
				AssistantSource: "system_error",
				ProviderError: &chatProviderError{
					Code:        startErr.Code,
					Message:     startErr.Message,
					Remediation: startErr.Remediation,
				},
			}, nil
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

func chatSuggestions(detail *chats.Detail) []string {
	if detail == nil || len(detail.Messages) == 0 {
		return []string{
			"Explore a larger task in a workspace",
			"Create a reusable agent for this kind of work",
			"Show me how this project is configured",
		}
	}
	last := detail.Messages[len(detail.Messages)-1]
	text := strings.ToLower(last.Content)
	switch {
	case strings.Contains(text, "agent"):
		return []string{
			"Help me design a reusable agent for this workflow",
			"Explain which agents would handle this",
			"Show how to customize the role flow",
		}
	case strings.Contains(text, "plan") || strings.Contains(text, "implement") || strings.Contains(text, "fix"):
		return []string{
			"Run this as a workspace task",
			"Explain the selected agent flow",
			"Summarize the current run state",
		}
	case strings.Contains(text, "setup") || strings.Contains(text, "config"):
		return []string{
			"Explain my model and tool setup",
			"Help me configure a model provider",
			"Show which agents and tools are ready to execute",
		}
	default:
		return []string{
			"Turn this into a long-horizon workspace task",
			"Design an agent for this recurring work",
			"Suggest concrete next steps",
		}
	}
}

func chatDirectReply(request *http.Request, services Services, content string, route *orchestration.RouteDecision) directChatResult {
	fallback := ""
	if route != nil {
		fallback = orchestration.DirectReply(*route)
	}
	if fallback == "" {
		fallback = "I can help here in chat. Ask a normal question, or describe a larger goal and I’ll open a workspace when it needs planning, files, tools, or agents."
	}
	if route == nil || route.Mode != orchestration.ModeDirectReply {
		return directChatResult{Content: fallback, Source: "clarification"}
	}
	if services.Adapter == nil || services.Secrets == nil {
		return directChatResult{
			Source: "system_error",
			ProviderError: &chatProviderError{
				Code:        "model_runtime_unavailable",
				Message:     "The model runtime is not available in Gateway.",
				Remediation: "Restart Gateway or run `nomici doctor`.",
			},
		}
	}
	profile, err := directReplyProfile(request, services, route)
	if err != nil || profile == nil {
		return directChatResult{
			Source: "system_error",
			ProviderError: &chatProviderError{
				Code:        "model_not_configured",
				Message:     "No model provider is connected.",
				Remediation: "Run `nomici setup`, or add a model in Settings.",
			},
		}
	}
	apiKey := ""
	if profile.APIKeyEnv != "" {
		resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
		if !ok {
			return directChatResult{
				Source:         "system_error",
				ModelProfileID: profile.ID,
				ProviderError: &chatProviderError{
					Code:        "model_auth_missing",
					Message:     fmt.Sprintf("The selected model provider needs environment variable `%s`.", profile.APIKeyEnv),
					Remediation: "Set the variable locally or update the model in Settings.",
					ProviderID:  profile.Kind,
					ModelID:     profile.Model,
				},
			}
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
			{Role: "system", Content: "You are Nomici, a concise local-first multi-agent workspace assistant. Answer directly in chat. Do not use formal task-intake wording unless the user is explicitly asking Nomici to run a larger task."},
			{Role: "user", Content: content},
		},
		Options: adapters.InvokeOptions{TimeoutMs: 30000},
	})
	if err != nil {
		return directChatResult{Source: "system_error", ModelProfileID: profile.ID, ProviderError: modelProviderError("model_invoke_failed", "The configured model provider did not respond.", "Run `nomici doctor` or check Settings > Models before retrying.", profile)}
	}
	if result == nil {
		return directChatResult{Source: "system_error", ModelProfileID: profile.ID, ProviderError: modelProviderError("model_empty_response", "The configured model provider returned an empty response.", "Check Settings > Models before retrying.", profile)}
	}
	if result.Status != adapters.StatusCompleted {
		return directChatResult{Source: "system_error", ModelProfileID: profile.ID, ProviderError: modelFailureError(result, profile)}
	}
	if len(result.Messages) == 0 {
		return directChatResult{Source: "system_error", ModelProfileID: profile.ID, ProviderError: modelProviderError("model_missing_message", "The configured model completed without a chat message.", "Check the selected model in Settings before retrying.", profile)}
	}
	reply := strings.TrimSpace(result.Messages[len(result.Messages)-1].Content)
	if reply == "" {
		return directChatResult{Source: "system_error", ModelProfileID: profile.ID, ProviderError: modelProviderError("model_empty_message", "The configured model returned an empty chat message.", "Check the selected model in Settings before retrying.", profile)}
	}
	return directChatResult{Content: reply, Source: "model", ModelProfileID: profile.ID}
}

func modelFailureError(result *adapters.InvokeResult, profile *providers.Profile) *chatProviderError {
	if result != nil && result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
		message := strings.Join(strings.Fields(result.Error.Message), " ")
		if len(message) > 180 {
			message = message[:177] + "..."
		}
		return modelProviderError(result.Error.Code, "The configured model could not answer: "+message, "Run `nomici doctor` or update the model in Settings.", profile)
	}
	return modelProviderError("model_failed", "The configured model could not answer.", "Run `nomici doctor` or update the model in Settings.", profile)
}

func modelProviderError(code string, message string, remediation string, profile *providers.Profile) *chatProviderError {
	err := &chatProviderError{
		Code:        code,
		Message:     message,
		Remediation: remediation,
	}
	if profile != nil {
		err.ProviderID = profile.Kind
		err.ModelID = profile.Model
	}
	return err
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
	return routeMetadataWithSource(route, "", "")
}

func routeMetadataWithSource(route *orchestration.RouteDecision, assistantSource string, modelProfileID string) json.RawMessage {
	payloadMap := map[string]any{}
	if route == nil {
		if assistantSource == "" && modelProfileID == "" {
			return json.RawMessage("{}")
		}
	} else {
		payloadMap["route_decision"] = route
	}
	if assistantSource != "" {
		payloadMap["assistant_source"] = assistantSource
	}
	if modelProfileID != "" {
		payloadMap["model_profile_id"] = modelProfileID
	}
	payload, err := json.Marshal(payloadMap)
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
