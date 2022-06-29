package api

import (
	"encoding/json"
	"net/http"

	"github.com/oursky/github-actions-manager/pkg/github/runners"
)

type runnerResponse struct {
	Epoch   int64
	Runners any
}

type instanceSummary struct {
	ID   int64
	Name string
}

func (s *Server) apiRunners(rw http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	isSummary := r.Form.Has("summary")
	isOnlineOnly := r.Form.Has("online")
	state := s.runners.State().Value()

	var instances []runners.Instance
	for _, i := range state.Instances {
		instances = append(instances, i)
		if isOnlineOnly && !i.IsOnline {
			continue
		}
	}

	var resp any = instances
	if isSummary {
		var summary []instanceSummary
		for _, i := range instances {
			summary = append(summary, instanceSummary{ID: i.ID, Name: i.Name})
		}
		resp = runnerResponse{Epoch: state.Epoch, Runners: summary}
	} else {
		resp = runnerResponse{Epoch: state.Epoch, Runners: instances}
	}

	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(200)
	json.NewEncoder(rw).Encode(resp)
}
