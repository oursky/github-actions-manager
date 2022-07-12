package jobs

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
	"github.com/google/go-github/v45/github"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/h2non/gock.v1"
)

func ptr[T any](v T) *T {
	return &v
}
func TestRun(t *testing.T) {

	logger, _ := zap.NewProduction()
	sync_page_size := 5
	webhook_server_addr := "127.0.0.1:8001"
	config := &Config{
		Disabled:          false,
		ReplayEnabled:     true,
		RetentionPeriod:   &tomltypes.Duration{1 * time.Hour},
		SyncInterval:      &tomltypes.Duration{5 * time.Second},
		SyncPageSize:      &sync_page_size,
		WebhookServerAddr: &webhook_server_addr,
		WebhookSecret:     "testing",
	}

	kv := kv.NewInMemoryStore()
	registry := prometheus.NewRegistry()
	client := &http.Client{Transport: &http.Transport{}}
	gock.InterceptClient(client)
	defer gock.Off()

	testGithubWorkflowRun := &github.WorkflowRun{
		ID:             ptr(int64(0)),
		Status:         ptr("succeed"),
		Conclusion:     ptr("succeed"),
		WorkflowID:     ptr(int64(0)),
		HeadCommit:     &github.HeadCommit{},
		HeadRepository: &github.Repository{},
	}

	testCommitMsg := testGithubWorkflowRun.GetHeadCommit().GetMessage()
	testCommitMsgTitle, _, _ := strings.Cut(testCommitMsg, "\n")
	testCommitURL := testGithubWorkflowRun.GetHeadRepository().GetHTMLURL() + "/commit/" + testGithubWorkflowRun.GetHeadCommit().GetID()

	testWorkflowRun := &WorkflowRun{
		Key: Key{ID: int64(0), RepoOwner: "tester", RepoName: "testing"},

		Name:       testGithubWorkflowRun.GetName(),
		URL:        testGithubWorkflowRun.GetHTMLURL(),
		Status:     testGithubWorkflowRun.GetStatus(),
		Conclusion: testGithubWorkflowRun.GetConclusion(),

		StartedAt:          testGithubWorkflowRun.GetRunStartedAt().Time,
		CommitMessageTitle: testCommitMsgTitle,
		CommitURL:          testCommitURL,
	}

	testGithubWorkflowJob := &github.WorkflowJob{
		ID:         ptr(int64(0)),
		HTMLURL:    ptr("testing"),
		Status:     ptr("succeed"),
		Conclusion: ptr("succeed"),
	}

	var startedAt *time.Time
	if gt := testGithubWorkflowJob.GetStartedAt(); !gt.IsZero() {
		startedAt = &gt.Time
	}

	var completedAt *time.Time
	if gt := testGithubWorkflowJob.GetCompletedAt(); !gt.IsZero() {
		completedAt = &gt.Time
	}

	testWorkflowJob := &WorkflowJob{
		Key: Key{ID: int64(0), RepoOwner: "tester", RepoName: "testing"},

		Name:       testGithubWorkflowJob.GetName(),
		URL:        testGithubWorkflowJob.GetHTMLURL(),
		Status:     testGithubWorkflowJob.GetStatus(),
		Conclusion: testGithubWorkflowJob.GetConclusion(),

		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		RunnerID:     testGithubWorkflowJob.RunnerID,
		RunnerName:   testGithubWorkflowJob.RunnerName,
		RunnerLabels: testGithubWorkflowJob.Labels,
	}

	testWorkflowRun.Jobs = append(testWorkflowRun.Jobs, testWorkflowJob)

	testWebhookJob := NewWebhookObject(
		Key{ID: int64(0), RepoOwner: "tester", RepoName: "testing"},
		testGithubWorkflowJob,
	)

	testWebhookRun := NewWebhookObject(
		Key{ID: int64(0), RepoOwner: "tester", RepoName: "testing"},
		testGithubWorkflowRun,
	)

	gock.New("https://api.github.com/repos/(.*)/(.*)/actions/jobs/(.*)").
		Persist().
		Reply(200).
		JSON(testGithubWorkflowJob)

	gock.New("https://api.github.com/repos/(.*)/(.*)/actions/runs/(.*)").
		Persist().
		Reply(200).
		JSON(testGithubWorkflowRun)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	s, _ := NewSynchronizer(logger, config, client, kv, registry, TestClock{})
	s.Start(ctx, g)
	s.webhookRuns <- testWebhookRun
	s.webhookJobs <- testWebhookJob
	time.Sleep(1 * time.Second)
	assert.Equal(t, testWorkflowRun, s.metrics.state.WorkflowRuns[0])

}
