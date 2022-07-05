package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type server struct {
	logger     *zap.Logger
	server     *http.Server
	managerAPI *managerAPI
	provider   Provider
}

func newServer(logger *zap.Logger, config *Config, managerAPI *managerAPI, gatherer prometheus.Gatherer, provider Provider) *server {
	logger = logger.Named("server")

	mux := http.NewServeMux()
	server := &server{
		logger: logger,
		server: &http.Server{
			Addr:         config.GetAddr(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			Handler:      mux,
			ErrorLog:     zap.NewStdLog(logger),
		},
		managerAPI: managerAPI,
		provider:   provider,
	}

	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{
		ErrorLog: zap.NewStdLog(logger.Named("prom")),
	}))

	apiMux := http.NewServeMux()
	mux.Handle("/api/v1/", useAuth(provider, apiMux))

	apiMux.HandleFunc("/api/v1/agent", server.apiAgentPost)
	apiMux.HandleFunc("/api/v1/agent/", server.apiAgentGetDelete)

	return server
}

func (s *server) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		go func() {
			<-ctx.Done()

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.server.Shutdown(shutdownCtx)
		}()

		s.logger.Info("starting server", zap.String("addr", s.server.Addr))
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("failed to run server: %w", err)
		}
		return nil
	})
	return nil
}
