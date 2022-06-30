package runners

import (
	"strconv"

	"github.com/oursky/github-actions-manager/pkg/utils/promutil"
	"github.com/prometheus/client_golang/prometheus"
)

type Instance struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	IsOnline bool     `json:"isOnline"`
	IsBusy   bool     `json:"isBusy"`
	Labels   []string `json:"labels"`
}

func (i *Instance) labels() prometheus.Labels {
	labels := prometheus.Labels{
		"runner_id":   strconv.FormatInt(i.ID, 10),
		"runner_name": i.Name,
	}
	for _, l := range i.Labels {
		labels["runner_label_"+promutil.SanitizeLabel(l)] = l
	}
	return labels
}

type State struct {
	Epoch     int64
	Instances map[string]Instance
}

func (s *State) Lookup(name string, id int64) (*Instance, bool) {
	if name == "" {
		return nil, false
	}
	if inst, ok := s.Instances[name]; ok {
		// id == 0 -> unknown, don't check ID
		if id == 0 || inst.ID == id {
			return &inst, true
		}
	}
	return nil, false
}
