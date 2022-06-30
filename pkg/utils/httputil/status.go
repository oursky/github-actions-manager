package httputil

import (
	"fmt"
	"net/http"
)

type ErrHTTPStatus int

func (s ErrHTTPStatus) Error() string {
	return fmt.Sprintf("unexpected status code: %d", s)
}

func CheckStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return ErrHTTPStatus(resp.StatusCode)
}
