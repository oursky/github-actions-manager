package dashboard

import (
	"net/http"
)

func (s *Server) styles(rw http.ResponseWriter, r *http.Request) {
	s.asset(rw, "styles.css", "text/css; charset=utf-8")
}
