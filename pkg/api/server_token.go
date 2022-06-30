package api

import (
	"net/http"

	"github.com/oursky/github-actions-manager/pkg/utils/httputil"
)

func (s *Server) apiToken(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "unsupported method", http.StatusBadRequest)
		return
	}

	token, err := s.regToken.Get(r.Context())
	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte(err.Error()))
		return
	}

	type resp struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	httputil.RespondJSON(rw, resp{
		Token: token,
		URL:   s.target.URL(),
	})
}
