package gateway

import (
	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	"github.com/NomiciAI/nomici-orchestrator/internal/blocked"
	"github.com/NomiciAI/nomici-orchestrator/internal/chats"
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway/web"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/memory"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/NomiciAI/nomici-orchestrator/internal/uploads"
	"github.com/NomiciAI/nomici-orchestrator/internal/worklocks"
	"github.com/go-chi/chi/v5"
)

type Services struct {
	Providers *providers.Store
	Trace     *trace.Store
	Secrets   *secrets.Resolver
	Adapter   *adapters.ModelAdapter
	Graph     *graph.Store
	Packs     *packs.Store
	Policy    *policy.Service
	Context   *sharedcontext.Store
	Runs      *runpkg.Store
	Sandboxes *sandbox.Store
	Artifacts *artifacts.Store
	Uploads   *uploads.Store
	Chats     *chats.Store
	Tools     *toolbroker.Store
	Memory    *memory.Store
	Blocked   *blocked.Store
	Locks     *worklocks.Store
}

func NewRouter(options Options, services Services) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/api/health", healthHandler(options))
	router.Group(func(api chi.Router) {
		if options.AuthToken != "" {
			api.Use(bearerAuthMiddleware(options.AuthToken))
		}
		api.Get("/api/console/overview", consoleOverviewHandler(options, services))
		api.Get("/api/provider-catalog", providerCatalogHandler())
		api.Get("/api/provider-catalog/{provider_id}/models", providerCatalogModelsHandler(options))
		api.Post("/api/provider-catalog/{provider_id}/doctor", providerCatalogDoctorHandler(options))
		api.Get("/api/chats", chatListHandler(services))
		api.Post("/api/chats", chatCreateHandler(options, services))
		api.Get("/api/chats/{chat_id}", chatDetailHandler(services))
		api.Post("/api/chats/{chat_id}/messages", chatMessageHandler(options, services))
		api.Get("/api/models", modelListHandler(services))
		api.Get("/api/packs", packListHandler(services))
		api.Get("/api/graphs/latest", latestGraphHandler(services))
		api.Get("/api/runtimes", runtimeListHandler(services))
		api.Get("/api/sessions", sessionListHandler(services))
		api.Get("/api/sessions/{session_id}", sessionDetailHandler(services))
		api.Get("/api/sessions/{session_id}/tasks", sessionTasksHandler(services))
		api.Post("/api/sessions/{session_id}/cancel", sessionCancelHandler(services))
		api.Post("/api/sessions/{session_id}/resume", sessionResumeHandler(options, services))
		api.Post("/api/sessions/{session_id}/plan/revise", sessionPlanReviseHandler(services))
		api.Post("/api/sessions/{session_id}/plan/approve", sessionPlanApproveHandler(options, services))
		api.Get("/api/sessions/{session_id}/blocked-actions", sessionBlockedActionsHandler(services))
		api.Post("/api/sessions/{session_id}/blocked-actions/{blocked_action_id}/resolve", sessionBlockedActionResolveHandler(options, services))
		api.Post("/api/sessions/{session_id}/clarifications", sessionClarificationHandler(options, services))
		api.Get("/api/review-queue", reviewQueueHandler(services))
		api.Get("/api/sessions/{session_id}/tool-calls", sessionToolCallsHandler(services))
		api.Post("/api/sessions/{session_id}/tool-calls", sessionToolCallCreateHandler(options, services))
		api.Get("/api/sessions/{session_id}/events", sessionEventsHandler(services))
		api.Get("/api/tools", toolListHandler())
		api.Get("/api/tools/{tool_id}", toolDetailHandler())
		api.Get("/api/skills", skillListHandler(options))
		api.Get("/api/skills/{skill_id}", skillDetailHandler(options))
		api.Get("/api/memory/proposals", memoryProposalListHandler(services))
		api.Get("/api/memory/items", memoryItemListHandler(services))
		api.Delete("/api/memory/items/{context_id}", memoryItemDeleteHandler(services))
		api.Post("/api/memory/proposals/{proposal_id}/approve", memoryProposalApproveHandler(services))
		api.Post("/api/memory/proposals/{proposal_id}/reject", memoryProposalRejectHandler(services))
		api.Post("/api/memory/proposals/{proposal_id}/delete", memoryProposalDeleteHandler(services))
		api.Get("/api/agents", agentListHandler(options, services))
		api.Post("/api/agents", agentCreateHandler(options, services))
		api.Get("/api/agents/{agent_id}", agentDetailHandler(options, services))
		api.Patch("/api/agents/{agent_id}", agentUpdateHandler(options, services))
		api.Delete("/api/agents/{agent_id}", agentDeleteHandler(options, services))
		api.Post("/api/agents/{agent_id}/validate", agentValidateHandler())
		api.Get("/api/orchestration", orchestrationShowHandler(options))
		api.Patch("/api/orchestration", orchestrationUpdateHandler(options))
		api.Post("/api/orchestration/validate", orchestrationValidateHandler())
		api.Get("/api/uploads", uploadListHandler(services))
		api.Post("/api/uploads", uploadCreateHandler(services))
		api.Get("/api/artifacts", artifactListHandler(services))
		api.Get("/api/artifacts/{artifact_id}", artifactDetailHandler(services))
		api.Get("/api/artifacts/{artifact_id}/revisions", artifactRevisionsHandler(services))
		api.Get("/api/artifacts/{artifact_id}/content", artifactContentHandler(services))
		api.Get("/api/artifacts/{artifact_id}/download", artifactDownloadHandler(services))
		api.Get("/api/runs", runListHandler(services))
		api.Post("/api/runs", runCreateHandler(options, services))
		api.Get("/api/runs/{run_id}", runDetailHandler(services))
		api.Get("/api/runs/{run_id}/events", runEventsHandler(services))
		api.Get("/api/approvals", approvalListHandler(services))
		api.Post("/api/approvals/{approval_id}/grant", approvalGrantHandler(services))
		api.Post("/api/approvals/{approval_id}/deny", approvalDenyHandler(services))
		if services.Providers != nil && services.Trace != nil && services.Secrets != nil && services.Adapter != nil {
			api.Post("/api/models/test", modelTestHandler(services))
		}
	})
	router.Group(func(v1 chi.Router) {
		if options.AuthToken != "" {
			v1.Use(bearerAuthMiddleware(options.AuthToken))
		}
		v1.Get("/v1/models", v1ModelsHandler(services))
		v1.Post("/v1/chat/completions", v1ChatCompletionsHandler(services))
	})
	router.Handle("/*", web.Handler())

	return router
}
