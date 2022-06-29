package api

import (
	"net/http"
)

func (s *Server) token(rw http.ResponseWriter, r *http.Request) {
	token, err := s.regToken.Get(r.Context())
	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte(token))
}
