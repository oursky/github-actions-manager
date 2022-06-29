package httputil

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

type KeyAuthMiddleware struct {
	next http.Handler
	keys []string
}

func UseKeyAuth(keys []string, next http.Handler) *KeyAuthMiddleware {
	return &KeyAuthMiddleware{
		next: next,
		keys: keys,
	}
}

func (m *KeyAuthMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	bearer, key, ok := strings.Cut(authz, " ")
	if !ok || strings.ToLower(bearer) != "bearer" {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("invalid key"))
		return
	}

	c := 0
	for _, k := range m.keys {
		c += subtle.ConstantTimeCompare([]byte(k), []byte(key))
	}
	if c == 0 {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("invalid key"))
		return
	}

	m.next.ServeHTTP(rw, r)
}
