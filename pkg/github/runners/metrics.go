package runners

import (
	"sync"

	"github.com/oursky/github-actions-manager/pkg/utils/promutil"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	state *State
	lock  *sync.RWMutex

	epoch  *promutil.MetricDesc
	busy   *promutil.MetricDesc
	online *promutil.MetricDesc
}

func newMetrics(state *State, r *prometheus.Registry) *metrics {
	m := &metrics{
		state: state,
		lock:  new(sync.RWMutex),

		epoch: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "runner",
			Name:      "epoch",
			Help:      "Number of epoches of runner synchronization.",
		}),
		busy: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "runner",
			Name:      "busy",
			Help:      "Describes whether the runner is busy.",
		}),
		online: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "runner",
			Name:      "online",
			Help:      "Describes whether the runner is online.",
		}),
	}
	r.MustRegister(m)
	return m
}

func (m *metrics) Describe(ch chan<- *prometheus.Desc) {}

func (m *metrics) Collect(ch chan<- prometheus.Metric) {
	state := m.get()

	ch <- m.epoch.Counter(float64(state.Epoch), nil)
	for _, i := range state.Instances {
		labels := i.labels()
		ch <- m.busy.GaugeBool(i.IsBusy, labels)
		ch <- m.online.GaugeBool(i.IsOnline, labels)
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
