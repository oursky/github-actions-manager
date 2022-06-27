package jobs

import (
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
)

type State struct {
	WorkflowRuns []*WorkflowRun
}

type WorkflowRun struct {
	ID        int64
	RepoOwner string
	RepoName  string

	Name       string
	URL        string
	Status     string
	Conclusion string

	CreatedAt          time.Time
	CommitMessageTitle string
	CommitURL          string

	Jobs []*WorkflowJob
}

type WorkflowJob struct {
	ID        int64
	RepoOwner string
	RepoName  string

	Name       string
	URL        string
	Status     string
	Conclusion string

	StartedAt    *time.Time
	CompletedAt  *time.Time
	RunnerID     *int64
	RunnerName   *string
	RunnerLabels []string
}

type cell[T any] struct {
	UpdatedAt time.Time
	Object    *T
}

func newState(runs map[Key]cell[github.WorkflowRun], jobs map[Key]cell[github.WorkflowJob]) *State {
	runMap := make(map[Key]*WorkflowRun)
	for key, c := range runs {
		run := c.Object
		commitMsg := run.GetHeadCommit().GetMessage()
		commitMsgTitle, _, _ := strings.Cut(commitMsg, "\n")
		commitURL := run.GetHeadRepository().GetHTMLURL() + "/commit/" + run.GetHeadCommit().GetID()

		runMap[key] = &WorkflowRun{
			ID:        key.ID,
			RepoOwner: key.RepoOwner,
			RepoName:  key.RepoName,

			Name:       run.GetName(),
			URL:        run.GetHTMLURL(),
			Status:     run.GetStatus(),
			Conclusion: run.GetConclusion(),

			CreatedAt:          run.GetCreatedAt().Time,
			CommitMessageTitle: commitMsgTitle,
			CommitURL:          commitURL,
		}
	}
	for key, c := range jobs {
		job := c.Object
		run, ok := runMap[Key{
			ID:        job.GetRunID(),
			RepoOwner: key.RepoOwner,
			RepoName:  key.RepoName,
		}]
		if !ok {
			continue
		}

		var startedAt *time.Time
		if t := job.GetStartedAt(); !t.IsZero() {
			startedAt = &t.Time
		}

		var completedAt *time.Time
		if t := job.GetCompletedAt(); !t.IsZero() {
			completedAt = &t.Time
		}

		run.Jobs = append(run.Jobs, &WorkflowJob{
			ID:        key.ID,
			RepoOwner: key.RepoOwner,
			RepoName:  key.RepoName,

			Name:       job.GetName(),
			URL:        job.GetHTMLURL(),
			Status:     job.GetStatus(),
			Conclusion: job.GetConclusion(),

			StartedAt:    startedAt,
			CompletedAt:  completedAt,
			RunnerID:     job.RunnerID,
			RunnerName:   job.RunnerName,
			RunnerLabels: job.Labels,
		})
	}

	state := &State{}
	for _, run := range runMap {
		sort.Slice(run.Jobs, func(i, j int) bool {
			return run.Jobs[i].ID < run.Jobs[j].ID
		})
		if len(run.Jobs) == 0 {
			continue
		}
		state.WorkflowRuns = append(state.WorkflowRuns, run)
	}
	sort.Slice(state.WorkflowRuns, func(i, j int) bool {
		return state.WorkflowRuns[i].CreatedAt.Before(state.WorkflowRuns[j].CreatedAt)
	})

	return state
}
