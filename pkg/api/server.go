package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"github.com/oursky/github-actions-manager/pkg/utils/httputil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type RunnersState interface {
	State() *channels.Broadcaster[*runners.State]
}

type Server struct {
	logger   *zap.Logger
	enabled  bool
	server   *http.Server
	runners  RunnersState
	target   github.Target
	regToken *github.RegistrationTokenStore
}

func NewServer(logger *zap.Logger, config *Config, runners RunnersState, target github.Target, gatherer prometheus.Gatherer) *Server {
	if config.Disabled {
		return &Server{enabled: false}
	}

	logger = logger.Named("api")

	r := mux.NewRouter()
	server := &Server{
		logger:  logger,
		enabled: true,
		server: &http.Server{
			Addr:         config.GetAddr(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			Handler:      r,
			ErrorLog:     zap.NewStdLog(logger),
		},
		runners:  runners,
		target:   target,
		regToken: github.NewRegistrationTokenStore(logger, target),
	}

	r.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{
		ErrorLog: zap.NewStdLog(logger.Named("prom")),
	}))

	apiR := mux.NewRouter()
	r.Handle("/api/v1/", httputil.UseKeyAuth(config.AuthKeys, apiR))

	apiR.HandleFunc("/api/v1/token", server.apiToken).Methods("GET")
	apiR.HandleFunc("/api/v1/runners", server.apiRunnersGet).Methods("GET")
	apiR.HandleFunc("/api/v1/runners/", server.apiRunnerDelete).Methods("DELETE")

	return server
}

func (s *Server) Start(ctx context.Context, g *errgroup.Group) error {
	if !s.enabled {
		return nil
	}

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
