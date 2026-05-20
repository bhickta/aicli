package newsapi

import (
	"context"
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
	req, ok := core.DecodeJSON[news.Request](w, r)
	if !ok {
		return
	}
	var p provider.Provider
	if req.UseLLM {
		selected, ok := h.runtime.ProviderOrError(w, req.ProviderID)
		if !ok {
			return
		}
		p = selected
	}
	h.runtime.StartJob(w, r, "news", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("loading and deduplicating news", 2, 4)
		result, err := news.New(p).Run(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}
