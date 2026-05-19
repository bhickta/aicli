package studyapi

import (
	"context"
	"encoding/json"
	"errors"
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
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		Notes      string `json:"notes"`
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

	job := core.NewJob("recall", req.Notes)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("generating recall triggers", 2, 4)
		result, err := recall.New(p).Generate(ctx, recall.Request{Model: req.Model, Notes: req.Notes})
		progress("saving triggers", 3, 4)
		return result, err
	})
}
