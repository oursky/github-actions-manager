package dashboard

import (
	"net/http"
	"sort"

	"github.com/oursky/github-actions-manager/pkg/github/runners"
)

type dataIndex struct {
	Runners []runners.Instance
}

func (s *Server) index(rw http.ResponseWriter, r *http.Request) {
	state := s.state.State()
	var runners []runners.Instance
	if state != nil {
		for _, i := range state.Instances {
			runners = append(runners, i)
		}
		sort.Slice(runners, func(i, j int) bool {
			return runners[i].ID < runners[j].ID
		})
	}

	data := &dataIndex{
		Runners: runners,
	}
	s.template(rw, "index.html", data)
}
