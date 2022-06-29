package jobs

import (
	"context"
	"net/http"
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/google/go-github/v45/github"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger *zap.Logger
	config *Config
	server *webhookServer
	github *github.Client

	state   *channels.Broadcaster[*State]
	metrics *metrics
}

func NewSynchronizer(logger *zap.Logger, config *Config, client *http.Client, registry *prometheus.Registry) (*Synchronizer, error) {
	logger = logger.Named("jobs-sync")

	server := newWebhookServer(logger, config.GetWebhookServerAddr(), config.WebhookSecret)

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
	runs := make(chan webhookObject[*github.WorkflowRun])
	jobs := make(chan webhookObject[*github.WorkflowJob])

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

func (s *Synchronizer) run(
	ctx context.Context,
	webhookRuns <-chan webhookObject[*github.WorkflowRun],
	webhookJobs <-chan webhookObject[*github.WorkflowJob],
) {
	runs := make(map[Key]cell[github.WorkflowRun])
	jobs := make(map[Key]cell[github.WorkflowJob])

	for {
		select {
		case <-ctx.Done():
			return

		case run := <-webhookRuns:
			runs[run.Key] = cell[github.WorkflowRun]{
				UpdatedAt: time.Now(),
				Object:    run.Object,
			}

		case job := <-webhookJobs:
			jobs[job.Key] = cell[github.WorkflowJob]{
				UpdatedAt: time.Now(),
				Object:    job.Object,
			}

			runKey := job.Key
			runKey.ID = job.Object.GetRunID()
			run, _, err := s.github.Actions.GetWorkflowRunByID(ctx, runKey.RepoOwner, runKey.RepoName, runKey.ID)
			if err != nil {
				s.logger.Warn("failed to get workflow run",
					zap.Error(err),
					zap.String("owner", runKey.RepoOwner),
					zap.String("repo", runKey.RepoName),
					zap.Int64("id", runKey.ID),
				)
			}

			runs[runKey] = cell[github.WorkflowRun]{
				UpdatedAt: time.Now(),
				Object:    run,
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
