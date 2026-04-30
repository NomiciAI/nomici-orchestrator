package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8787
)

type Options struct {
	Host    string
	Port    int
	Version string
}

type Server struct {
	options    Options
	httpServer *http.Server
}

func NewServer(options Options) *Server {
	if options.Host == "" {
		options.Host = DefaultHost
	}
	if options.Port == 0 {
		options.Port = DefaultPort
	}

	return &Server{
		options: options,
		httpServer: &http.Server{
			Addr:              net.JoinHostPort(options.Host, strconv.Itoa(options.Port)),
			Handler:           NewRouter(options),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (server *Server) Run(ctx context.Context) error {
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
