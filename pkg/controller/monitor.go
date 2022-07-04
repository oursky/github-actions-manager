package controller

import (
	"context"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github/runners"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type monitor struct {
	logger            *zap.Logger
	syncInterval      time.Duration
	transitionTimeout time.Duration
	managerAPI        *managerAPI
	provider          Provider
}

func newMonitor(logger *zap.Logger, config *Config, managerAPI *managerAPI, provider Provider) *monitor {
	return &monitor{
		logger:            logger.Named("monitor"),
		syncInterval:      config.GetSyncInterval(),
		transitionTimeout: config.GetTransitionTimeout(),
		managerAPI:        managerAPI,
		provider:          provider,
	}
}

func (m *monitor) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		m.run(ctx)
		return nil
	})
	return nil
}

func (m *monitor) run(ctx context.Context) {
	epoch := int64(0)
	stopping := false
	for !stopping {
		m.check(ctx, &epoch)

		select {
		case <-ctx.Done():
			stopping = true

		case <-time.After(m.syncInterval):
		}
	}

	if m.provider.Capabilities().KeepAgentsOnExit {
		return
	}

	ctx = context.Background()
	for {
		if ok := m.terminateAll(ctx); ok {
			m.logger.Info("all agents terminated")
			return
		}

		m.check(ctx, &epoch)

		time.Sleep(m.syncInterval)
	}
}

func (m *monitor) terminateAll(ctx context.Context) bool {
	agents, err := m.provider.State().Agents()
	if err != nil {
		m.logger.Warn("failed to get agents", zap.Error(err))
		return false
	}

	if len(agents) == 0 {
		return true
	}

	m.logger.Info("terminating agents", zap.Int("count", len(agents)))
	now := time.Now()
	terminator := func(agent *Agent) {
		m.transition(agent, AgentStateTerminating, now)
	}
	for _, a := range agents {
		if a.State == AgentStateTerminating {
			continue
		}
		if err := m.provider.State().UpdateAgent(a.ID, terminator); err != nil {
			m.logger.Warn("failed to terminate agent", zap.Error(err), zap.String("id", a.ID))
		}
	}
	return false
}

func (m *monitor) check(ctx context.Context, epoch *int64) {
	serverEpoch, instances, err := m.managerAPI.GetRunners(ctx)
	if err != nil {
		m.logger.Warn("failed to get runners", zap.Error(err))
		return
	}
	newEpoch := serverEpoch != *epoch
	*epoch = serverEpoch

	now := time.Now()
	agents, err := m.provider.State().Agents()
	if err != nil {
		m.logger.Warn("failed to get agents", zap.Error(err))
		return
	}

	m.logger.Debug("checking runners", zap.Int("count", len(agents)))
	for _, a := range agents {
		agent := a
		var instance *runners.Instance
		if i, ok := instances[agent.RunnerName]; ok {
			if agent.RunnerID == nil || *agent.RunnerID == i.ID {
				instance = &i
			}
		}

		if err := m.provider.CheckAgent(ctx, &agent, instance); err != nil {
			m.logger.Warn("failed to check agent", zap.Error(err), zap.String("id", agent.ID))
			continue
		}
		if err := m.checkAgent(ctx, &agent, now, newEpoch, instance); err != nil {
			m.logger.Warn("failed to check agent", zap.Error(err), zap.String("id", agent.ID))
		}
	}
}

func (m *monitor) checkAgent(
	ctx context.Context,
	agent *Agent,
	now time.Time,
	newEpoch bool,
	instance *runners.Instance,
) error {
	switch agent.State {
	case AgentStatePending:
		return m.checkTimeout(agent, now, newEpoch)

	case AgentStateConfiguring:
		if instance != nil {
			return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
				agent.RunnerID = new(int64)
				*agent.RunnerID = instance.ID
				if !instance.IsOnline {
					m.transition(agent, AgentStateStarting, now)
				} else {
					m.transition(agent, AgentStateReady, now)
				}
			})
		}
		return m.checkTimeout(agent, now, newEpoch)

	case AgentStateStarting:
		if instance == nil {
			m.logger.Info("agent is gone", zap.String("id", agent.ID))
			return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
				m.transition(agent, AgentStateTerminating, now)
			})
		} else if instance.IsOnline {
			return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
				m.transition(agent, AgentStateReady, now)
			})
		}
		return m.checkTimeout(agent, now, newEpoch)

	case AgentStateReady:
		if instance == nil {
			m.logger.Info("agent is gone", zap.String("id", agent.ID))
			return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
				m.transition(agent, AgentStateTerminating, now)
			})
		}
		if !instance.IsOnline {
			m.logger.Info("agent is offline", zap.String("id", agent.ID))
			return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
				m.transition(agent, AgentStateStarting, now)
			})
		}
		return nil

	case AgentStateTerminating:
		dead := true
		if instance != nil {
			dead = false
			if newEpoch {
				m.logger.Info("deleting runner",
					zap.Int64("runnerID", instance.ID),
					zap.String("runnerName", instance.Name),
				)
				if err := m.managerAPI.DeleteRunner(ctx, instance.ID); err != nil {
					if now.Sub(agent.LastTransitionTime) > m.transitionTimeout {
						m.logger.Warn("failed to delete agent, abandoning")
						dead = true
					}
				} else {
					dead = true
				}
			}
		}

		if err := m.provider.TerminateAgent(ctx, *agent); err != nil {
			m.logger.Info("failed to terminate agent", zap.Error(err))
			dead = false
		}

		if dead {
			m.logger.Info("cleaning up agent", zap.String("id", agent.ID))
			return m.provider.State().DeleteAgent(agent.ID)
		}
		return nil
	default:
		panic("unreachable")
	}
}

func (m *monitor) checkTimeout(agent *Agent, now time.Time, newEpoch bool) error {
	if now.Sub(agent.LastTransitionTime) < m.transitionTimeout {
		return nil
	}
	// wait for new epoch to confirm timeout
	if !newEpoch {
		return nil
	}
	// timed out
	m.logger.Info("agent timed out",
		zap.String("id", agent.ID),
		zap.String("runnerName", agent.RunnerName),
	)
	return m.provider.State().UpdateAgent(agent.ID, func(agent *Agent) {
		m.transition(agent, AgentStateTerminating, now)
	})
}

func (m *monitor) transition(agent *Agent, state AgentState, timestamp time.Time) {
	m.logger.Debug("agent state transition",
		zap.String("id", agent.ID),
		zap.String("state", string(state)),
	)
	agent.State = state
	agent.LastTransitionTime = timestamp
}
