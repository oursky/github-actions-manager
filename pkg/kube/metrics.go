package kube

import (
	"regexp"
	"strconv"

	"github.com/oursky/github-actions-manager/pkg/utils/promutil"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

type metrics struct {
	state *ControllerState

	kubePod                  *promutil.MetricDesc
	statefulSetBusyRunnerOrd *promutil.MetricDesc
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

		statefulSetBusyRunnerOrd: promutil.NewMetricDesc(prometheus.Opts{
			Namespace: "github_actions",
			Subsystem: "kube",
			Name:      "stateful_set_busy_runner_ord",
			Help:      "The highest ordinal of busy runners pod in StatefulSet.",
		}),
	}
	return m
}

func (m *metrics) Describe(ch chan<- *prometheus.Desc) {}

func (m *metrics) Collect(ch chan<- prometheus.Metric) {
	pods, _ := m.state.pods.List(labels.SelectorFromSet(labels.Set{
		labelRunner: "true",
	}))

	type statefulSetPod struct {
		*v1.Pod
		Ord int
	}
	statefulSetBusyRunnerOrds := make(map[string]statefulSetPod)

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

		if pod.Annotations[annotationBusy] == "true" {
			ctrl := metav1.GetControllerOf(pod)
			if ctrl != nil && ctrl.Kind == "StatefulSet" {
				parent, ord := getStatefulSetPodInfo(pod)
				if parent != "" && statefulSetBusyRunnerOrds[parent].Ord <= ord {
					statefulSetBusyRunnerOrds[parent] = statefulSetPod{
						Pod: pod,
						Ord: ord,
					}
				}
			}
		}
	}

	for set, info := range statefulSetBusyRunnerOrds {
		ch <- m.statefulSetBusyRunnerOrd.Gauge(float64(info.Ord), prometheus.Labels{
			"statefulset": set,
			"namespace":   info.Pod.Namespace,
		})
	}
}

func getStatefulSetPodInfo(pod *v1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}
