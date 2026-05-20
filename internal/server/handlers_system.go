package server

import (
	"net/http"

	"github.com/bhickta/aicli/internal/systemresources"
)

func (s *Server) systemResources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, systemresources.Collect(r.Context()))
}
