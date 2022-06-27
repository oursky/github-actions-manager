package runners

import (
	"context"
	"sync"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger  *zap.Logger
	config  *Config
	target  github.Target
	lock    *sync.RWMutex
	state   *State
	waiters []chan<- *State
}

func NewSynchronizer(logger *zap.Logger, config *Config, target github.Target) *Synchronizer {
	return &Synchronizer{
		logger:  logger.Named("runner-sync"),
		config:  config,
		target:  target,
		lock:    new(sync.RWMutex),
		state:   nil,
		waiters: nil,
	}
}

func (s *Synchronizer) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		s.run(ctx)
		return nil
	})
	return nil
}

func (s *Synchronizer) Wait() <-chan *State {
	c := make(chan *State, 1)
	func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		s.waiters = append(s.waiters, c)
	}()

	return c
}

func (s *Synchronizer) State() *State {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.state
}

func (s *Synchronizer) next(state *State) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.state = state
	for _, c := range s.waiters {
		c <- state
	}
	s.waiters = nil
}

func (s *Synchronizer) run(ctx context.Context) {
	syncInterval := s.config.GetSyncInterval()

	work := &syncWork{Synchronizer: s, pageSize: s.config.GetSyncPageSize()}
	work.reset(1)

	for {
		state := work.do(ctx)
		if state != nil {
			s.next(state)
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
	s.logger.Info("fetching page", zap.Int("page", s.page))
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

	s.logger.Info("synchronized runners",
		zap.Int64("epoch", s.epoch),
		zap.Time("beginTime", s.beginTime),
		zap.Int("count", len(s.instances)),
	)

	return &State{Epoch: s.epoch, Instances: s.instances}
}
