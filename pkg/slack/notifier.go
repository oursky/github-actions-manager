package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackutilsx"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type JobsState interface {
	State() *channels.Broadcaster[*jobs.State]
}

type Notifier struct {
	logger *zap.Logger
	app    *App
	client *github.Client
	jobs   JobsState
}

func NewNotifier(logger *zap.Logger, app *App, client *github.Client, jobs JobsState) *Notifier {
	logger = logger.Named("slack-notifier")
	return &Notifier{
		logger: logger,
		app:    app,
		client: client,
		jobs:   jobs,
	}
}

func (n *Notifier) Start(ctx context.Context, g *errgroup.Group) error {
	if n.app.Disabled() {
		return nil
	}

	g.Go(func() error {
		n.run(ctx)
		return nil
	})
	return nil
}

func (n *Notifier) run(ctx context.Context) {
	runStatuses := make(map[jobs.Key]string)
	sub := channels.NewSubscriber(ctx, n.jobs.State())

	for {
		select {
		case <-ctx.Done():
			return

		case s := <-sub.Wait():
			if s == nil {
				continue
			}
			n.logger.Debug("new job state", zap.Int("count", len(s.WorkflowRuns)))

			runKeys := make(map[jobs.Key]struct{})
			for _, run := range s.WorkflowRuns {
				runKeys[run.Key] = struct{}{}
				status := runStatuses[run.Key]
				if run.Status != status {
					n.logger.Info("status updated",
						zap.String("repo", run.RepoName),
						zap.String("status", run.Status),
						zap.String("conclusion", run.Conclusion),
					)
					n.notify(ctx, run)
					runStatuses[run.Key] = run.Status
				}
			}

			for key := range runStatuses {
				if _, ok := runKeys[key]; !ok {
					delete(runStatuses, key)
				}
			}
		}
	}
}

func (n *Notifier) notify(ctx context.Context, run *jobs.WorkflowRun) {
	if run.Status != "completed" {
		return
	}

	repo := fmt.Sprintf("%s/%s", run.RepoOwner, run.RepoName)
	channels, err := n.app.GetChannels(ctx, repo)
	if err != nil {
		n.logger.Warn("failed to get channels", zap.Error(err), zap.String("repo", repo))
		return
	}
	if len(channels) == 0 {
		return
	}

	runtime := "-"
	timing, _, err := n.client.Actions.GetWorkflowRunUsageByID(ctx, run.RepoOwner, run.RepoName, run.ID)
	if err != nil {
		n.logger.Warn("failed to get timing", zap.Error(err))
	} else {
		runtime = (time.Millisecond * time.Duration(timing.GetRunDurationMS())).String()
	}

	const colorGreen = "#16a34a"  // green-600
	const colorYellow = "#d97706" // amber-600
	const colorRed = "#7f1d1d"    // red-900
	const colorGray = "#94a3b8"   // slate-400

	var msg string = ""
	var color string = colorGray
	switch run.Conclusion {
	case "action_required":
		msg = fmt.Sprintf("%s requires action.", run.Name)
		color = colorYellow
	case "cancelled":
		msg = fmt.Sprintf("%s is cancelled.", run.Name)
	case "skipped":
		msg = ""
	case "failure":
		msg = fmt.Sprintf("%s has failed in %s.", run.Name, runtime)
		color = colorRed
	case "timed_out":
		msg = fmt.Sprintf("%s timed out in %s.", run.Name, runtime)
		color = colorYellow
	case "success":
		msg = fmt.Sprintf("%s has succeeded in %s.", run.Name, runtime)
		color = colorGreen
	default:
		msg = fmt.Sprintf("%s has completed in %s.", run.Name, runtime)
	}

	if msg == "" {
		return
	}

	slackMsg := slack.Attachment{
		Color:      color,
		Title:      msg,
		TitleLink:  run.URL,
		AuthorName: repo,
		MarkdownIn: []string{"fields"},
		Fields: []slack.AttachmentField{{
			Title: "Commit",
			Value: fmt.Sprintf(
				"<%s|%s>",
				slackutilsx.EscapeMessage(run.CommitURL),
				slackutilsx.EscapeMessage(run.CommitMessageTitle),
			),
		}},
	}

	for _, channel := range channels {
		if !channel.filter.Any(run) {
			return
		}
		err := n.app.SendMessage(ctx, channel.channelID, slack.MsgOptionAttachments(slackMsg))
		if err != nil {
			n.logger.Warn("failed to send message",
				zap.Error(err),
				zap.String("channelID", channel.channelID),
			)
		}
	}
}
