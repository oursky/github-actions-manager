package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/oursky/github-actions-manager/pkg/utils/channels"

	"github.com/google/go-github/v45/github"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Replayer struct {
	logger *zap.Logger
	source Source

	retentionPeriod time.Duration
	syncInterval    time.Duration
	pageSize        int
}

func NewReplayer(
	logger *zap.Logger,
	source Source,
	retentionPeriod time.Duration,
	syncInterval time.Duration,
	pageSize int,
) *Replayer {
	return &Replayer{
		logger:          logger.Named("webhook-replayer"),
		source:          source,
		retentionPeriod: retentionPeriod,
		syncInterval:    syncInterval,
		pageSize:        pageSize,
	}
}

func (r *Replayer) Start(ctx context.Context, g *errgroup.Group, runs chan<- Key, jobs chan<- Key) error {
	g.Go(func() error {
		r.run(ctx, runs, jobs)
		return nil
	})
	return nil
}

func (r *Replayer) run(ctx context.Context, runs chan<- Key, jobs chan<- Key) {
	work := &replayWork{
		Replayer:       r,
		retentionLimit: time.Now().Add(-r.retentionPeriod),
		cursor:         "",
		runs:           runs,
		jobs:           jobs,
		seenRuns:       make(map[Key]struct{}),
		seenJobs:       make(map[Key]struct{}),
	}

	for {
		finished, err := work.do(ctx)
		if err != nil {
			r.logger.Warn("failed to replay webhook", zap.Error(err))
		} else if finished {
			r.logger.Info("caught up past deliveries")
			return
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(r.syncInterval):
			continue
		}
	}
}

type replayWork struct {
	*Replayer
	retentionLimit time.Time
	cursor         string
	runs           chan<- Key
	jobs           chan<- Key
	seenRuns       map[Key]struct{}
	seenJobs       map[Key]struct{}
}

func (r *replayWork) do(ctx context.Context) (bool, error) {
	deliveries, next, err := r.source.listDeliveries(ctx, r.cursor, r.pageSize)
	if err != nil {
		return false, fmt.Errorf("failed to list deliveries: %w", err)
	}

	for _, d := range deliveries {
		id := d.GetID()
		timestamp := d.GetDeliveredAt()
		if timestamp.Before(r.retentionLimit) {
			return true, nil
		}

		if d.GetEvent() != "workflow_job" && d.GetEvent() != "workflow_run" {
			continue
		}
		r.logger.Debug("getting delivery", zap.Int64("id", id), zap.String("guid", d.GetGUID()))
		d, err := r.source.getDelivery(ctx, id)
		if err != nil {
			return false, fmt.Errorf("failed to get #%d: %w", id, err)
		}

		payload, err := d.ParseRequestPayload()
		if err != nil {
			return false, fmt.Errorf("failed to parse #%d: %w", id, err)
		}

		switch payload := payload.(type) {
		case *github.WorkflowRunEvent:
			key := Key{
				ID:        payload.GetWorkflowRun().GetID(),
				RepoOwner: payload.GetRepo().GetOwner().GetLogin(),
				RepoName:  payload.GetRepo().GetName(),
			}

			if _, seen := r.seenRuns[key]; !seen {
				r.seenRuns[key] = struct{}{}
				if err := channels.Send(ctx, r.runs, key); err != nil {
					return false, err
				}
			}
		case *github.WorkflowJobEvent:
			key := Key{
				ID:        payload.GetWorkflowJob().GetID(),
				RepoOwner: payload.GetRepo().GetOwner().GetLogin(),
				RepoName:  payload.GetRepo().GetName(),
			}

			if _, seen := r.seenJobs[key]; !seen {
				r.seenJobs[key] = struct{}{}
				if err := channels.Send(ctx, r.jobs, key); err != nil {
					return false, err
				}
			}
		}
	}

	r.cursor = next
	return false, nil
}
