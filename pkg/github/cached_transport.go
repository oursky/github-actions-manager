package github

import (
	"net/http"

	"github.com/gregjones/httpcache"
)

type cachedTransport struct {
	rt http.RoundTripper
}

func NewCachedTransport(rt http.RoundTripper) http.RoundTripper {
	return &httpcache.Transport{
		Cache:     httpcache.NewMemoryCache(),
		Transport: &cachedTransport{rt: rt},
	}
}

func (t *cachedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(r)
	if err != nil {
		return resp, err
	}
	// Ensure API is is always called & checked for freshness
	resp.Header.Del("Cache-Control")
	return resp, err
}
