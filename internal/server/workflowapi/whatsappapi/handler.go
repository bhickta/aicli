package whatsappapi

import (
	"context"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/whatsapp"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/whatsapp/schedule", h.schedule)
}

func (h *Handler) schedule(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[whatsapp.ScheduleRequest](w, r)
	if !ok {
		return
	}
	input := req.RecipientName
	if input == "" {
		input = req.RecipientPhone
	}
	if input == "" {
		input = req.Recipient
	}
	h.runtime.StartJob(w, r, "whatsapp-scheduled-message", input, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return whatsapp.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Schedule(ctx, req, progress)
	})
}
