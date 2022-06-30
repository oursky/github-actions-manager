package agent

import (
	"context"

	"github.com/oursky/github-actions-manager/pkg/controller"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Agent struct {
	logger *zap.Logger
	config *Config

	executer *executer
	watcher  *watcher
}

func NewAgent(logger *zap.Logger, config *Config, provider Provider) *Agent {
	logger = logger.Named("agent")
	controllerAPI := newControllerAPI(provider)

	agentCh := make(chan controller.Agent, 1)

	return &Agent{
		logger:   logger,
		config:   config,
		executer: newExecuter(logger, config, controllerAPI, provider, agentCh),
		watcher:  newWatcher(logger, config, controllerAPI, agentCh),
	}
}

func (a *Agent) Start(ctx context.Context, g *errgroup.Group) error {
	if err := a.executer.Start(ctx, g); err != nil {
		return err
	}
	if err := a.watcher.Start(ctx, g); err != nil {
		return err
	}
	return nil
}
