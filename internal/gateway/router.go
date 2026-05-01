package gateway

import (
	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway/web"
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
}

func NewRouter(options Options, services Services) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/api/health", healthHandler(options))
	if services.Providers != nil && services.Trace != nil && services.Secrets != nil && services.Adapter != nil {
		router.Post("/api/models/test", modelTestHandler(services))
	}
	router.Handle("/*", web.Handler())

	return router
}
