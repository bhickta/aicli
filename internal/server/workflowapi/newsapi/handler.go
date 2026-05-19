package newsapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/workflow/news"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/news/run", h.runNews)
}

func (h *Handler) runNews(w http.ResponseWriter, r *http.Request) {
	var req news.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var p provider.Provider
	if req.UseLLM {
		selected, ok := h.runtime.ProviderFor(req.ProviderID)
		if !ok {
			core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
			return
		}
		p = selected
	}
	job := core.NewJob("news", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("loading and deduplicating news", 2, 4)
		result, err := news.New(p).Run(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}
