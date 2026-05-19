package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/workflow/image"
)

func (s *Server) runImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.Request
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
	job := s.newJob("image", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("analyzing image with vision model", 2, 4)
		result, err := image.New(p).Run(ctx, req.Request)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runImageRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.RenameRequest
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
	job := s.newJob("image-rename", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("planning safe rename", 2, 4)
		result, err := image.New(p).Rename(ctx, req.RenameRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runImagePruneRefs(w http.ResponseWriter, r *http.Request) {
	var req image.PruneRefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("image-prune-refs", req.MarkdownPath)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("checking referenced assets", 2, 4)
		result, err := image.PruneRefs(req)
		progress("saving result", 3, 4)
		return result, err
	})
}
