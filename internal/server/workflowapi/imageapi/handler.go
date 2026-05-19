package imageapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/workflow/image"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/image/run", h.runImage)
	mux.HandleFunc("POST /api/workflows/image/rename", h.runImageRename)
	mux.HandleFunc("POST /api/workflows/image/prune-refs", h.runImagePruneRefs)
}

func (h *Handler) runImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.Request
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("image", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("analyzing image with vision model", 2, 4)
		result, err := image.New(p).Run(ctx, req.Request)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runImageRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.RenameRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("image-rename", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("planning safe rename", 2, 4)
		result, err := image.New(p).Rename(ctx, req.RenameRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runImagePruneRefs(w http.ResponseWriter, r *http.Request) {
	var req image.PruneRefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("image-prune-refs", req.MarkdownPath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("checking referenced assets", 2, 4)
		result, err := image.PruneRefs(req)
		progress("saving result", 3, 4)
		return result, err
	})
}
