package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func urlParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func urlParamInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// intPtrOrZero returns *p if p is non-nil, otherwise 0. Used for request
// fields that are *int so we can tell "omitted" (nil) from "explicitly 0".
func intPtrOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
