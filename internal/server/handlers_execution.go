package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/execution"
)

var (
	errExecutionDisabled     = errors.New("authenticated execution API is not configured")
	errExecutionUnauthorized = errors.New("invalid execution service token")
)

func (s *Server) registerExecutionRoutes() {
	s.mux.Handle("POST /api/execution/run", s.executionAuth(http.HandlerFunc(s.runExecution)))
	s.mux.Handle("GET /api/execution/control", s.executionAuth(http.HandlerFunc(s.executionControl)))
	s.mux.Handle("PUT /api/execution/profiles", s.executionAuth(http.HandlerFunc(s.updateExecutionProfiles)))
	s.mux.Handle("PUT /api/execution/providers", s.executionAuth(http.HandlerFunc(s.updateExecutionProviders)))
	s.mux.Handle("GET /api/execution/models", s.executionAuth(http.HandlerFunc(s.executionModels)))
	s.mux.Handle("POST /api/execution/health", s.executionAuth(http.HandlerFunc(s.executionHealth)))
}

func (s *Server) runExecution(w http.ResponseWriter, r *http.Request) {
	var request execution.Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	response, err := s.execution.Execute(r.Context(), request)
	if err != nil {
		writeError(w, executionStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func executionStatus(err error) int {
	switch {
	case errors.Is(err, execution.ErrProfileNotFound):
		return http.StatusNotFound
	case errors.Is(err, execution.ErrDisabled), errors.Is(err, execution.ErrCapability):
		return http.StatusForbidden
	case errors.Is(err, execution.ErrNoTargets):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}
