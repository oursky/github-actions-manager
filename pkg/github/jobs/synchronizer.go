package jobs

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github/jobs/webhook"

	"github.com/google/go-github/v45/github"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger   *zap.Logger
	config   *Config
	replayer *webhook.Replayer
	server   *webhook.Server
	github   *github.Client

	lock    *sync.RWMutex
	state   *State
	waiters []chan<- *State
}

func NewSynchronizer(logger *zap.Logger, config *Config, client *http.Client) (*Synchronizer, error) {
	logger = logger.Named("jobs-sync")

	var replayer *webhook.Replayer
	if config.ReplayEnabled {
		source, err := webhook.NewSource(client)
		if err != nil {
			return nil, fmt.Errorf("failed to create webhook source: %w", err)
		}
		replayer = webhook.NewReplayer(
			logger,
			source,
			config.GetRetentionPeriod(),
			config.GetSyncInterval(),
			config.GetSyncPageSize(),
		)
	}

	server := webhook.NewServer(logger, config.GetWebhookServerAddr(), config.WebhookSecret)

	return &Synchronizer{
		logger:   logger,
		config:   config,
		replayer: replayer,
		server:   server,
		github:   github.NewClient(client),
		lock:     new(sync.RWMutex),
		state:    nil,
		waiters:  nil,
	}, nil
}

func (s *Synchronizer) Start(ctx context.Context, g *errgroup.Group) error {
	runs := make(chan webhook.Key)
	jobs := make(chan webhook.Key)

	if s.replayer != nil {
		s.replayer.Start(ctx, g, runs, jobs)
	}
	s.server.Start(ctx, g, runs, jobs)
	g.Go(func() error {
		s.run(ctx, runs, jobs)
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

func (s *Synchronizer) run(ctx context.Context, runKeys <-chan webhook.Key, jobKeys <-chan webhook.Key) {
	runs := make(map[Key]cell[github.WorkflowRun])
	jobs := make(map[Key]cell[github.WorkflowJob])

	for {
		select {
		case <-ctx.Done():
			return

		case key := <-runKeys:
			run, _, err := s.github.Actions.GetWorkflowRunByID(ctx, key.RepoOwner, key.RepoName, key.ID)
			if err != nil {
				s.logger.Warn("failed to get workflow run",
					zap.String("owner", key.RepoOwner),
					zap.String("repo", key.RepoName),
					zap.Int64("id", key.ID),
				)
				break
			}

			runs[Key{ID: key.ID, RepoOwner: key.RepoOwner, RepoName: key.RepoName}] = cell[github.WorkflowRun]{
				UpdatedAt: time.Now(),
				Object:    run,
			}

		case key := <-jobKeys:
			job, _, err := s.github.Actions.GetWorkflowJobByID(ctx, key.RepoOwner, key.RepoName, key.ID)
			if err != nil {
				s.logger.Warn("failed to get workflow job",
					zap.String("owner", key.RepoOwner),
					zap.String("repo", key.RepoName),
					zap.Int64("id", key.ID),
				)
				break
			}

			jobs[Key{ID: key.ID, RepoOwner: key.RepoOwner, RepoName: key.RepoName}] = cell[github.WorkflowJob]{
				UpdatedAt: time.Now(),
				Object:    job,
			}

		}

		retentionLimit := time.Now().Add(-s.config.GetRetentionPeriod())

		runRefs := make(map[Key]int)
		for key, job := range jobs {
			if job.UpdatedAt.Before(retentionLimit) {
				delete(jobs, key)
				continue
			}

			runKey := Key{ID: job.Object.GetRunID(), RepoOwner: key.RepoOwner, RepoName: key.RepoName}
			runRefs[runKey]++
		}

		for key, run := range runs {
			if run.UpdatedAt.Before(retentionLimit) {
				delete(runs, key)
				continue
			}
		}

		state := newState(runs, jobs)
		s.next(state)
	}
}
