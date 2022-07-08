package controller

import "net/http"

type ProviderAuthMiddleware struct {
	provider Provider
}

func NewProviderAuthMiddleware(provider Provider) *ProviderAuthMiddleware {
	return &ProviderAuthMiddleware{
		provider: provider,
	}
}

func (m *ProviderAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		m.provider.AuthenticateRequest(rw, r, next)
	})
}
