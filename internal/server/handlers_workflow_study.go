package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/workflow/recall"
)

func (s *Server) runRecall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}

	job := s.newJob("recall", req.Notes)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("generating recall triggers", 2, 4)
		result, err := recall.New(p).Generate(ctx, recall.Request{Model: req.Model, Notes: req.Notes})
		progress("saving triggers", 3, 4)
		return result, err
	})
}
