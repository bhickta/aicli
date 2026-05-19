package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/news"
)

func (s *Server) runNews(w http.ResponseWriter, r *http.Request) {
	var req news.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var p provider.Provider
	if req.UseLLM {
		selected, ok := s.providerFor(req.ProviderID)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("provider not found"))
			return
		}
		p = selected
	}
	job := s.newJob("news", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("loading and deduplicating news", 2, 4)
		result, err := news.New(p).Run(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}
