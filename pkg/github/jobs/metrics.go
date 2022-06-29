package jobs

import (
	"sync"

	"github.com/oursky/github-actions-manager/pkg/utils/promutil"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	state *State
	lock  *sync.RWMutex

	statusQueued     *promutil.MetricDesc
	statusInProgress *promutil.MetricDesc
	statusCompleted  *promutil.MetricDesc
	startedAt        *promutil.MetricDesc
	completedAt      *promutil.MetricDesc
}

func newMetrics(r *prometheus.Registry) *metrics {
	m := &metrics{
		state: nil,
		lock:  new(sync.RWMutex),

		statusQueued: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "job",
			Name:      "status_queued",
			Help:      "Describes whether the job is queued.",
		}),
		statusInProgress: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "job",
			Name:      "status_in_progress",
			Help:      "Describes whether the job is in progress.",
		}),
		statusCompleted: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "job",
			Name:      "status_completed",
			Help:      "Describes whether the job is completed.",
		}),
		startedAt: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "job",
			Name:      "start_time",
			Help:      "Start time in unix timestamp for a job.",
		}),
		completedAt: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "job",
			Name:      "completion_time",
			Help:      "Completion time in unix timestamp for a job.",
		}),
	}
	r.MustRegister(m)
	return m
}

func (m *metrics) Describe(ch chan<- *prometheus.Desc) {}

func (m *metrics) Collect(ch chan<- prometheus.Metric) {
	state := m.get()
	if state == nil {
		return
	}

	for _, run := range state.WorkflowRuns {
		for _, job := range run.Jobs {
			labels := job.labels()
			ch <- m.statusQueued.GaugeBool(job.Status == "queued", labels)
			ch <- m.statusInProgress.GaugeBool(job.Status == "in_progress", labels)
			ch <- m.statusCompleted.GaugeBool(job.Status == "completed", labels)
			if job.StartedAt != nil {
				ch <- m.startedAt.Gauge(float64(job.StartedAt.Unix()), labels)
			}
			if job.CompletedAt != nil {
				ch <- m.completedAt.Gauge(float64(job.CompletedAt.Unix()), labels)
			}
		}
	}
}

func (m *metrics) get() *State {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.state
}

func (m *metrics) update(state *State) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.state = state
}
