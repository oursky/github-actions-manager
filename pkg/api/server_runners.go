package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/oursky/github-actions-manager/pkg/github/runners"

	"go.uber.org/zap"
)

type runnerResponse struct {
	Epoch   int64              `json:"epoch"`
	Runners []runners.Instance `json:"runners"`
}

func (s *Server) apiRunnerDelete(rw http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	idstr := params["id"]

	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.target.DeleteRunner(r.Context(), id)
	if err != nil {
		s.logger.Warn("failed to delete runner", zap.Error(err), zap.Int64("id", id))
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	rw.WriteHeader(200)
}

func (s *Server) apiRunnersGet(rw http.ResponseWriter, r *http.Request) {
	state := s.runners.State().Value()

	var instances []runners.Instance
	for _, i := range state.Instances {
		instances = append(instances, i)
	}
	resp := runnerResponse{Epoch: state.Epoch, Runners: instances}

	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(200)
	json.NewEncoder(rw).Encode(resp)
}
