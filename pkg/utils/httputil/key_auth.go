package httputil

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

type KeyAuthMiddleware struct {
	keys []string
}

func NewKeyAuthMiddleware(keys []string) *KeyAuthMiddleware {
	return &KeyAuthMiddleware{
		keys: keys,
	}
}

func (m *KeyAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		bearer, key, ok := strings.Cut(authz, " ")
		if !ok || !strings.EqualFold(bearer, "Bearer") {
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
		next.ServeHTTP(rw, r)
	})

}
