package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) executionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := strings.TrimSpace(s.deps.ExecutionToken)
		if expected == "" {
			writeError(w, http.StatusServiceUnavailable, errExecutionDisabled)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			writeError(w, http.StatusUnauthorized, errExecutionUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
