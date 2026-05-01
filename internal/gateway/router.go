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
	router.Get("/api/console/overview", consoleOverviewHandler(options, services))
	router.Get("/api/models", modelListHandler(services))
	router.Get("/api/packs", packListHandler(services))
	router.Get("/api/graphs/latest", latestGraphHandler(services))
	router.Get("/api/runtimes", runtimeListHandler(services))
	router.Get("/api/runs", runListHandler(services))
	router.Get("/api/approvals", approvalListHandler(services))
	if services.Providers != nil && services.Trace != nil && services.Secrets != nil && services.Adapter != nil {
		router.Post("/api/models/test", modelTestHandler(services))
	}
	router.Handle("/*", web.Handler())

	return router
}
