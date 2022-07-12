package agent

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/oursky/github-actions-manager/pkg/controller"
	"github.com/oursky/github-actions-manager/pkg/utils/httputil"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type watcher struct {
	logger        *zap.Logger
	config        *Config
	controllerAPI *controllerAPI
	agentCh       <-chan controller.Agent
}

func newWatcher(logger *zap.Logger, config *Config, controllerAPI *controllerAPI, agentCh <-chan controller.Agent) *watcher {
	return &watcher{
		logger:        logger.Named("watcher"),
		config:        config,
		controllerAPI: controllerAPI,
		agentCh:       agentCh,
	}
}

func (w *watcher) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		w.run(ctx)
		return nil
	})
	return nil
}

func (w *watcher) run(ctx context.Context) error {
	var agent controller.Agent
	select {
	case <-ctx.Done():
		return nil
	case agent = <-w.agentCh:
	}

	needTermination := w.wait(ctx, agent)

	for needTermination {
		w.logger.Info("terminating agent", zap.String("id", agent.ID))
		err := w.controllerAPI.TerminateAgent(context.Background(), agent.ID)

		var errStatus httputil.ErrHTTPStatus
		if errors.As(err, &errStatus) && errStatus == http.StatusNotFound {
			needTermination = false
		} else if err != nil {
			w.logger.Warn("failed to terminate agent", zap.Error(err), zap.String("id", agent.ID))
			time.Sleep(5 * time.Second)
		} else {
			needTermination = false
		}
	}

	return nil
}

func (w *watcher) wait(ctx context.Context, agent controller.Agent) bool {
	w.logger.Info("watching agent status", zap.String("id", agent.ID))
	watchInterval := w.config.GetWatchInterval()

	for {
		select {
		case <-ctx.Done():
			return true

		case <-w.agentCh:
			return true

		case <-time.After(watchInterval):
		}

		name, state, err := w.controllerAPI.GetAgent(ctx, agent.ID)
		var errStatus httputil.ErrHTTPStatus
		if errors.As(err, &errStatus) && errStatus == http.StatusNotFound {
			w.logger.Info("agent not found")
			return false
		} else if err != nil {
			w.logger.Warn("failed to get agent status", zap.Error(err))
		}

		if name != agent.RunnerName {
			w.logger.Info("runner name mismatched")
			return false
		} else if state == controller.AgentStateTerminating {
			w.logger.Info("terminating agent")
			return false
		}
	}
}
