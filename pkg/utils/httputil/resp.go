package httputil

import (
	"encoding/json"
	"net/http"
)

func RespondJSON(rw http.ResponseWriter, resp any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(resp)
}
