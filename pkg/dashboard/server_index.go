package dashboard

import (
	"net/http"
	"sort"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/github/runners"
)

type dataIndex struct {
	Runners      []runners.Instance
	WorkflowRuns []*jobs.WorkflowRun
	RunnerJobMap map[int64]*jobs.WorkflowJob
}

func (s *Server) index(rw http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	isAll := r.Form.Has("all")

	rState := s.runners.State().Value()
	var runners []runners.Instance
	if rState != nil {
		for _, i := range rState.Instances {
			runners = append(runners, i)
		}
		sort.Slice(runners, func(i, j int) bool {
			return runners[i].ID < runners[j].ID
		})
	}

	jState := s.jobs.State().Value()
	var runs []*jobs.WorkflowRun
	jobMap := make(map[int64]*jobs.WorkflowJob)
	if jState != nil {
		for _, run := range jState.WorkflowRuns {
			if !isAll && run.Status == "completed" {
				continue
			}

			runs = append(runs, run)
			for _, job := range run.Jobs {
				if job.RunnerID != nil {
					jobMap[*job.RunnerID] = job
				}
			}
		}
	}

	data := &dataIndex{
		Runners:      runners,
		WorkflowRuns: runs,
		RunnerJobMap: jobMap,
	}
	s.template(rw, "index.html", data)
}
