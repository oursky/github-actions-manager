package jobs

import (
	"context"
	"net/http"
	"time"

	"github.com/oursky/github-actions-manager/pkg/github/jobs/webhook"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/google/go-github/v45/github"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger *zap.Logger
	config *Config
	server *webhook.Server
	github *github.Client

	state   *channels.Broadcaster[*State]
	metrics *metrics
}

func NewSynchronizer(logger *zap.Logger, config *Config, client *http.Client, registry *prometheus.Registry) (*Synchronizer, error) {
	logger = logger.Named("jobs-sync")

	server := webhook.NewServer(logger, config.GetWebhookServerAddr(), config.WebhookSecret)

	return &Synchronizer{
		logger:  logger,
		config:  config,
		server:  server,
		github:  github.NewClient(client),
		state:   channels.NewBroadcaster[*State](nil),
		metrics: newMetrics(registry),
	}, nil
}

func (s *Synchronizer) Start(ctx context.Context, g *errgroup.Group) error {
	runs := make(chan webhook.Key)
	jobs := make(chan webhook.Key)

	s.server.Start(ctx, g, runs, jobs)
	g.Go(func() error {
		s.run(ctx, runs, jobs)
		return nil
	})
	return nil
}

func (s *Synchronizer) State() *channels.Broadcaster[*State] {
	return s.state
}

func (s *Synchronizer) run(ctx context.Context, runKeys <-chan webhook.Key, jobKeys <-chan webhook.Key) {
	runs := make(map[Key]cell[github.WorkflowRun])
	jobs := make(map[Key]cell[github.WorkflowJob])

	for {
		select {
		case <-ctx.Done():
			return

		case key := <-runKeys:
			if _, err := s.updateRun(ctx, key, runs); err != nil {
				s.logger.Warn("failed to get workflow run",
					zap.Error(err),
					zap.String("owner", key.RepoOwner),
					zap.String("repo", key.RepoName),
					zap.Int64("id", key.ID),
				)
			}

		case key := <-jobKeys:
			job, err := s.updateJob(ctx, key, jobs)
			if err != nil {
				s.logger.Warn("failed to get workflow job",
					zap.Error(err),
					zap.String("owner", key.RepoOwner),
					zap.String("repo", key.RepoName),
					zap.Int64("id", key.ID),
				)
				break
			}

			runKey := webhook.Key{ID: job.GetRunID(), RepoOwner: key.RepoOwner, RepoName: key.RepoName}
			if _, err := s.updateRun(ctx, runKey, runs); err != nil {
				s.logger.Warn("failed to get workflow run",
					zap.Error(err),
					zap.String("owner", key.RepoOwner),
					zap.String("repo", key.RepoName),
					zap.Int64("id", key.ID),
				)
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
		s.state.Publish(state)
		s.metrics.update(state)
	}
}

func (s *Synchronizer) updateRun(ctx context.Context, key webhook.Key, runs map[Key]cell[github.WorkflowRun]) (*github.WorkflowRun, error) {
	run, _, err := s.github.Actions.GetWorkflowRunByID(ctx, key.RepoOwner, key.RepoName, key.ID)
	if err != nil {
		return nil, err
	}

	runs[Key{ID: key.ID, RepoOwner: key.RepoOwner, RepoName: key.RepoName}] = cell[github.WorkflowRun]{
		UpdatedAt: time.Now(),
		Object:    run,
	}

	return run, nil
}

func (s *Synchronizer) updateJob(ctx context.Context, key webhook.Key, jobs map[Key]cell[github.WorkflowJob]) (*github.WorkflowJob, error) {
	job, _, err := s.github.Actions.GetWorkflowJobByID(ctx, key.RepoOwner, key.RepoName, key.ID)
	if err != nil {
		return nil, err
	}

	jobs[Key{ID: key.ID, RepoOwner: key.RepoOwner, RepoName: key.RepoName}] = cell[github.WorkflowJob]{
		UpdatedAt: time.Now(),
		Object:    job,
	}

	return job, nil
}
