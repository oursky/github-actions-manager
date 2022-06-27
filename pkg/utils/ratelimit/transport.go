package ratelimit

import (
	"net/http"

	"golang.org/x/time/rate"
)

type Transport struct {
	Base        http.RoundTripper
	RateLimiter *rate.Limiter
}

func NewTransport(base http.RoundTripper, limit rate.Limit) *Transport {
	return &Transport{
		Base:        base,
		RateLimiter: rate.NewLimiter(limit, 1),
	}
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.RateLimiter.Wait(r.Context()); err != nil {
		if r.Body != nil {
			r.Body.Close()
		}
		return nil, err
	}
	return t.Base.RoundTrip(r)
}
