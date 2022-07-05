package kube

import (
	"strconv"

	"github.com/oursky/github-actions-manager/pkg/utils/promutil"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
)

type metrics struct {
	state *ControllerState

	kubePod *promutil.MetricDesc
}

func newMetrics(state *ControllerState) *metrics {
	m := &metrics{
		state: state,

		kubePod: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "kube",
			Name:      "pod",
			Help:      "Describes the associated pod of runner.",
		}),
	}
	return m
}

func (m *metrics) Describe(ch chan<- *prometheus.Desc) {}

func (m *metrics) Collect(ch chan<- prometheus.Metric) {
	pods, _ := m.state.pods.List(labels.SelectorFromSet(labels.Set{
		labelRunner: "true",
	}))

	for _, pod := range pods {
		agent := m.state.decodeState(pod)
		if agent == nil || agent.RunnerID == nil {
			continue
		}

		ch <- m.kubePod.Gauge(1, prometheus.Labels{
			"runner_id":   strconv.FormatInt(*agent.RunnerID, 10),
			"runner_name": agent.RunnerName,
			"pod":         pod.Name,
			"namespace":   pod.Namespace,
			"node":        pod.Spec.NodeName,
		})
	}
}
