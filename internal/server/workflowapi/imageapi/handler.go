package imageapi

import (
	"context"
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
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		image.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "image", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("analyzing image with vision model"))
		result, err := image.New(p).Run(ctx, req.Request)
		return result, err
	})
}

func (h *Handler) runImageRename(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		image.RenameRequest
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "image-rename", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("planning safe rename"))
		result, err := image.New(p).Rename(ctx, req.RenameRequest)
		return result, err
	})
}

func (h *Handler) runImagePruneRefs(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[image.PruneRefsRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "image-prune-refs", req.MarkdownPath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("checking referenced assets"))
		result, err := image.PruneRefs(req)
		return result, err
	})
}
