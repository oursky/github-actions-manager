package controller

import "net/http"

func useAuth(p Provider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		p.AuthenticateRequest(rw, r, next)
	})
}
