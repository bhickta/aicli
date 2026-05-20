package studyapi

import (
	"context"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/workflow/recall"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/recall/run", h.runRecall)
}

func (h *Handler) runRecall(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		Notes      string `json:"notes"`
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}

	h.runtime.StartJob(w, r, "recall", req.Notes, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("generating recall triggers", 2, 4)
		result, err := recall.New(p).Generate(ctx, recall.Request{Model: req.Model, Notes: req.Notes})
		progress("saving triggers", 3, 4)
		return result, err
	})
}
