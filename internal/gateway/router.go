package gateway

import (
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway/web"
	"github.com/go-chi/chi/v5"
)

func NewRouter(options Options) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/api/health", healthHandler(options))
	router.Handle("/*", web.Handler())

	return router
}
