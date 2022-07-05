package controller

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Controller struct {
	logger *zap.Logger
	config *Config

	server  *server
	monitor *monitor
}

func NewController(logger *zap.Logger, config *Config, gatherer prometheus.Gatherer, provider Provider) *Controller {
	managerAPI := newManagerAPI(config)
	server := newServer(logger, config, managerAPI, gatherer, provider)
	monitor := newMonitor(logger, config, managerAPI, provider)

	return &Controller{
		logger:  logger.Named("controller"),
		config:  config,
		server:  server,
		monitor: monitor,
	}
}

func (c *Controller) Start(ctx context.Context, g *errgroup.Group) error {
	if err := c.server.Start(ctx, g); err != nil {
		return err
	}
	if err := c.monitor.Start(ctx, g); err != nil {
		return err
	}
	return nil
}
