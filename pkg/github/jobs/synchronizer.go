package jobs

import (
	"context"
	"fmt"
	"math/rand"
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

type workState struct {
	runs map[Key]cell[github.WorkflowRun]
	jobs map[Key]cell[github.WorkflowJob]
}

func (s workState) setRun(owner string, repo string, r *github.WorkflowRun) {
	key := Key{RepoOwner: owner, RepoName: repo, ID: r.GetID()}
	cell := s.runs[key]
	updatedAt := r.GetUpdatedAt().Time
	if updatedAt.After(cell.UpdatedAt) {
		cell.Object = r
		cell.UpdatedAt = updatedAt
		s.runs[key] = cell
	}
}

func (s workState) setJob(owner string, repo string, j *github.WorkflowJob) {
	key := Key{RepoOwner: owner, RepoName: repo, ID: j.GetID()}
	cell := s.jobs[key]
	updatedAt := j.GetCompletedAt().Time
	if updatedAt.IsZero() {
		updatedAt = j.GetStartedAt().Time
	}
	if updatedAt.After(cell.UpdatedAt) {
		cell.Object = j
		cell.UpdatedAt = updatedAt
		s.jobs[key] = cell
	}
}

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
	st := workState{
		runs: make(map[Key]cell[github.WorkflowRun]),
		jobs: make(map[Key]cell[github.WorkflowJob]),
	}

	s.loadState(ctx, st)

	syncInterval := s.config.GetSyncInterval()

	for {
		select {
		case <-ctx.Done():
			return

		case o := <-webhookRuns:
			st.setRun(o.RepoOwner, o.RepoName, o.Object)

		case o := <-webhookJobs:
			st.setJob(o.RepoOwner, o.RepoName, o.Object)

			run, _, err := s.github.Actions.GetWorkflowRunByID(ctx, o.RepoOwner, o.RepoName, o.Object.GetRunID())
			if err != nil {
				s.logger.Warn("failed to get workflow run",
					zap.Error(err),
					zap.String("owner", o.RepoOwner),
					zap.String("repo", o.RepoName),
					zap.Int64("id", o.Object.GetRunID()),
				)
				break
			}

			st.setRun(o.RepoOwner, o.RepoName, run)

		case <-time.After(syncInterval):
			if len(st.runs) == 0 {
				continue
			}
			// 0: runs; 1: jobs
			choosenType := rand.Intn(2)
			switch choosenType {
			case 0:
				var choosenKey Key
				for key := range st.runs {
					choosenKey = key
					break
				}
				updatedRun, _, err := s.github.Actions.GetWorkflowRunByID(ctx, choosenKey.RepoOwner, choosenKey.RepoName, choosenKey.ID)
				if err != nil {
					s.logger.Warn("failed to get workflow run",
						zap.Error(err),
						zap.String("owner", choosenKey.RepoOwner),
						zap.String("repo", choosenKey.RepoName),
						zap.Int64("id", choosenKey.ID),
					)
				}
				st.setRun(choosenKey.RepoOwner, choosenKey.RepoName, updatedRun)
			case 1:
				var choosenKey Key
				for key := range st.jobs {
					choosenKey = key
					break
				}
				updatedJob, _, err := s.github.Actions.GetWorkflowJobByID(ctx, choosenKey.RepoOwner, choosenKey.RepoName, choosenKey.ID)
				if err != nil {
					s.logger.Warn("failed to get workflow job",
						zap.Error(err),
						zap.String("owner", choosenKey.RepoOwner),
						zap.String("repo", choosenKey.RepoName),
						zap.Int64("id", choosenKey.ID),
					)
				}
				st.setJob(choosenKey.RepoOwner, choosenKey.RepoName, updatedJob)
			}
		}

		retentionLimit := time.Now().Add(-s.config.GetRetentionPeriod())

		runRefs := make(map[Key]int)
		for key, job := range st.jobs {
			if job.UpdatedAt.Before(retentionLimit) {
				delete(st.jobs, key)
				continue
			}

			runKey := Key{ID: job.Object.GetRunID(), RepoOwner: key.RepoOwner, RepoName: key.RepoName}
			runRefs[runKey]++
		}

		for key, run := range st.runs {
			if run.UpdatedAt.Before(retentionLimit) {
				delete(st.runs, key)
				continue
			}
		}

		state := newState(st.runs, st.jobs)
		s.state.Publish(state)
		s.metrics.update(state)
		s.saveState(ctx, st.runs)
	}
}

func (s *Synchronizer) loadState(ctx context.Context, st workState) {
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
		st.setRun(owner, repo, wrun)

		wjobs, _, err := s.github.Actions.ListWorkflowJobs(
			ctx, owner, repo, id,
			&github.ListWorkflowJobsOptions{ListOptions: github.ListOptions{PerPage: 100}},
		)
		if err != nil {
			s.logger.Warn("failed to refresh state", zap.Error(err), zap.String("key", k))
			continue
		}
		for _, job := range wjobs.Jobs {
			st.setJob(owner, repo, job)
		}
	}

	s.logger.Info("reloaded state", zap.Int("runs", len(st.runs)), zap.Int("jobs", len(st.jobs)))
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
