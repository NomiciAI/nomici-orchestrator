package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8787
)

type Options struct {
	Host    string
	Port    int
	Version string
	DBPath  string
}

type Server struct {
	options    Options
	httpServer *http.Server
	db         *sql.DB
}

func NewServer(options Options) *Server {
	if options.Host == "" {
		options.Host = DefaultHost
	}
	if options.Port == 0 {
		options.Port = DefaultPort
	}

	if options.DBPath == "" {
		options.DBPath = store.DefaultDBPath
	}

	return &Server{options: options}
}

func (server *Server) Run(ctx context.Context) error {
	if err := server.initialize(); err != nil {
		return err
	}
	defer server.db.Close()

	errs := make(chan error, 1)

	go func() {
		log.Printf("Nomici Gateway listening on http://%s", server.httpServer.Addr)
		if err := server.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown gateway: %w", err)
		}
		return <-errs
	case err := <-errs:
		return err
	}
}

func (server *Server) initialize() error {
	db, err := store.Open(server.options.DBPath)
	if err != nil {
		return err
	}
	if err := store.Migrate(db); err != nil {
		_ = db.Close()
		return err
	}
	server.db = db

	services := Services{
		Providers: providers.NewStore(db),
		Trace:     trace.NewStore(db),
		Secrets:   secrets.NewResolver(),
		Adapter:   adapters.NewOpenAICompatibleAdapter(),
		Graph:     graph.NewStore(db),
		Packs:     packs.NewStore(db),
		Policy:    policy.NewService(db),
	}
	server.httpServer = &http.Server{
		Addr:              net.JoinHostPort(server.options.Host, strconv.Itoa(server.options.Port)),
		Handler:           NewRouter(server.options, services),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return nil
}
