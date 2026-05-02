package gateway

import (
	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway/web"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/go-chi/chi/v5"
)

type Services struct {
	Providers *providers.Store
	Trace     *trace.Store
	Secrets   *secrets.Resolver
	Adapter   *adapters.OpenAICompatibleAdapter
	Graph     *graph.Store
	Packs     *packs.Store
	Policy    *policy.Service
}

func NewRouter(options Options, services Services) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/api/health", healthHandler(options))
	router.Group(func(api chi.Router) {
		if options.AuthToken != "" {
			api.Use(bearerAuthMiddleware(options.AuthToken))
		}
		api.Get("/api/console/overview", consoleOverviewHandler(options, services))
		api.Get("/api/models", modelListHandler(services))
		api.Post("/api/models", modelSetupHandler(services))
		api.Get("/api/packs", packListHandler(services))
		api.Post("/api/packs/{packID}/install", packInstallHandler(options, services))
		api.Get("/api/graphs/latest", latestGraphHandler(services))
		api.Get("/api/runtimes", runtimeListHandler(services))
		api.Get("/api/runs", runListHandler(services))
		api.Post("/api/runs/agent", agentRunHandler(services))
		api.Get("/api/approvals", approvalListHandler(services))
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
