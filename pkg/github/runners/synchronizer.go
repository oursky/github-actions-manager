package runners

import (
	"context"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"github.com/prometheus/client_golang/prometheus"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger  *zap.Logger
	config  *Config
	target  github.Target
	state   *channels.Broadcaster[*State]
	metrics *metrics
}

func NewSynchronizer(logger *zap.Logger, config *Config, target github.Target, registry *prometheus.Registry) *Synchronizer {
	return &Synchronizer{
		logger:  logger.Named("runner-sync"),
		config:  config,
		target:  target,
		state:   channels.NewBroadcaster[*State](nil),
		metrics: newMetrics(registry),
	}
}

func (s *Synchronizer) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		s.run(ctx)
		return nil
	})
	return nil
}

func (s *Synchronizer) State() *channels.Broadcaster[*State] {
	return s.state
}

func (s *Synchronizer) run(ctx context.Context) {
	syncInterval := s.config.GetSyncInterval()

	work := &syncWork{Synchronizer: s, pageSize: s.config.GetSyncPageSize()}
	work.reset(1)

	for {
		state := work.do(ctx)
		if state != nil {
			s.state.Publish(state)
			s.metrics.update(state)
			work.reset(state.Epoch + 1)
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(syncInterval):
			continue
		}
	}
}

type syncWork struct {
	*Synchronizer
	pageSize int

	epoch     int64
	beginTime time.Time
	instances map[string]Instance
	page      int
}

func (s *syncWork) reset(epoch int64) {
	s.epoch = epoch
	s.beginTime = time.Now()
	s.instances = make(map[string]Instance)
	s.page = 1
}

func (s *syncWork) do(ctx context.Context) *State {
	s.logger.Debug("fetching page", zap.Int("page", s.page))
	runnersPage, nextPage, err := s.target.GetRunners(ctx, s.page, s.pageSize)
	if err != nil {
		s.logger.Warn("failed to get runners", zap.Error(err))
		return nil
	}

	for _, r := range runnersPage {
		labels := make([]string, len(r.Labels))
		for i, lbl := range r.Labels {
			labels[i] = lbl.GetName()
		}

		s.instances[r.GetName()] = Instance{
			ID:       r.GetID(),
			Name:     r.GetName(),
			IsOnline: r.GetStatus() == "online",
			IsBusy:   r.GetBusy(),
			Labels:   labels,
		}
	}

	if nextPage != 0 {
		s.page = nextPage
		return nil
	}

	s.logger.Debug("synchronized runners",
		zap.Int64("epoch", s.epoch),
		zap.Time("beginTime", s.beginTime),
		zap.Int("count", len(s.instances)),
	)

	return &State{Epoch: s.epoch, Instances: s.instances}
}
