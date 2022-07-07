package jobs

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	gh "github.com/oursky/github-actions-manager/pkg/github"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"

	"github.com/google/go-github/v45/github"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Synchronizer struct {
	logger *zap.Logger
	config *Config
	server *webhookServer
	github *github.Client
	kv     kv.Store

	state   *channels.Broadcaster[*State]
	metrics *metrics
}

func NewSynchronizer(logger *zap.Logger, config *Config, client *http.Client, kv kv.Store, registry *prometheus.Registry) (*Synchronizer, error) {
	logger = logger.Named("jobs-sync")

	server := newWebhookServer(logger, config.GetWebhookServerAddr(), config.WebhookSecret)

	return &Synchronizer{
		logger:  logger,
		config:  config,
		server:  server,
		github:  github.NewClient(client),
		kv:      kv,
		state:   channels.NewBroadcaster[*State](nil),
		metrics: newMetrics(registry),
	}, nil
}

func (s *Synchronizer) Start(ctx context.Context, g *errgroup.Group) error {
	if s.config.Disabled {
		return nil
	}

	runs := make(chan webhookObject[*github.WorkflowRun])
	jobs := make(chan webhookObject[*github.WorkflowJob])

	if err := s.server.Start(ctx, g, runs, jobs); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
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

	s.loadState(ctx, runs, jobs)

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
		s.saveState(ctx, runs)
	}
}

func (s *Synchronizer) loadState(
	ctx context.Context,
	runs map[Key]cell[github.WorkflowRun],
	jobs map[Key]cell[github.WorkflowJob],
) {
	data, err := s.kv.Get(ctx, gh.KVNamespace, KVKey)
	if err != nil {
		s.logger.Warn("failed to load state", zap.Error(err))
	}
	if len(data) == 0 {
		return
	}

	for _, k := range strings.Split(data, ";") {
		parts := strings.Split(k, "/")
		if len(parts) != 3 {
			s.logger.Warn("failed to load state", zap.Error(err))
			continue
		}

		owner := parts[0]
		repo := parts[1]
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			s.logger.Warn("failed to load state", zap.Error(err))
			continue
		}

		wrun, _, err := s.github.Actions.GetWorkflowRunByID(ctx, owner, repo, id)
		if err != nil {
			s.logger.Warn("failed to refresh state", zap.Error(err), zap.String("key", k))
			continue
		}
		runs[Key{RepoOwner: owner, RepoName: repo, ID: id}] = cell[github.WorkflowRun]{
			UpdatedAt: time.Now(),
			Object:    wrun,
		}

		wjobs, _, err := s.github.Actions.ListWorkflowJobs(
			ctx, owner, repo, id,
			&github.ListWorkflowJobsOptions{ListOptions: github.ListOptions{PerPage: 100}},
		)
		if err != nil {
			s.logger.Warn("failed to refresh state", zap.Error(err), zap.String("key", k))
			continue
		}
		for _, job := range wjobs.Jobs {
			jobs[Key{RepoOwner: owner, RepoName: repo, ID: job.GetID()}] = cell[github.WorkflowJob]{
				UpdatedAt: time.Now(),
				Object:    job,
			}
		}
	}

	s.logger.Info("reloaded state", zap.Int("runs", len(runs)), zap.Int("jobs", len(jobs)))
}

func (s *Synchronizer) saveState(ctx context.Context, runs map[Key]cell[github.WorkflowRun]) {
	var runKeys []string
	for key := range runs {
		runKeys = append(runKeys, fmt.Sprintf("%s/%s/%d", key.RepoOwner, key.RepoName, key.ID))
	}

	if err := s.kv.Set(ctx, gh.KVNamespace, KVKey, strings.Join(runKeys, ";")); err != nil {
		s.logger.Warn("failed to save state", zap.Error(err))
	}
}
