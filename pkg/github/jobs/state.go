package jobs

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/prometheus/client_golang/prometheus"
)

type State struct {
	WorkflowRuns []*WorkflowRun
}

type Key struct {
	ID        int64
	RepoOwner string
	RepoName  string
}

type WorkflowRun struct {
	Key

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
	Key

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

func (j *WorkflowJob) labels() prometheus.Labels {
	labels := prometheus.Labels{
		"workflow_job_id":  strconv.FormatInt(j.ID, 10),
		"repository_owner": j.RepoOwner,
		"repository_name":  j.RepoName,
		"name":             j.Name,
	}
	if j.RunnerID != nil {
		labels["runner_id"] = strconv.FormatInt(*j.RunnerID, 10)
	}
	if j.RunnerName != nil {
		labels["runner_name"] = *j.RunnerName
	}
	return labels
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
			Key: key,

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
			Key: key,

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
			return compareJob(run.Jobs[i], run.Jobs[j])
		})
		if len(run.Jobs) == 0 {
			continue
		}
		state.WorkflowRuns = append(state.WorkflowRuns, run)
	}
	sort.Slice(state.WorkflowRuns, func(i, j int) bool {
		return compareJob(state.WorkflowRuns[i].Jobs[0], state.WorkflowRuns[j].Jobs[0])
	})

	return state
}

var statusOrder map[string]int = map[string]int{
	"in_progress": 3,
	"queued":      2,
	"completed":   1,
}

var conclusionOrder map[string]int = map[string]int{
	"failure": 2,
	"success": 1,
}

func compareJob(a *WorkflowJob, b *WorkflowJob) bool {
	aKey := statusOrder[a.Status]
	bKey := statusOrder[b.Status]
	if aKey != bKey {
		return aKey > bKey
	}

	aKey = conclusionOrder[a.Conclusion]
	bKey = conclusionOrder[b.Conclusion]
	if aKey != bKey {
		return aKey > bKey
	}

	return a.ID < b.ID
}
