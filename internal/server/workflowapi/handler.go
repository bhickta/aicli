package workflowapi

import (
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/audioapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/codexapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/server/workflowapi/documentapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/imageapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/newsapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/studyapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/videoapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/whatsappapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/zettelapi"
)

type Dependencies = core.Dependencies

type Handler struct {
	runtime *core.Runtime
}

func New(deps Dependencies) *Handler {
	return &Handler{runtime: core.New(deps)}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/jobs/{id}/cancel", h.cancelJob)
	studyapi.New(h.runtime).Register(mux)
	codexapi.New(h.runtime).Register(mux)
	imageapi.New(h.runtime).Register(mux)
	newsapi.New(h.runtime).Register(mux)
	documentapi.New(h.runtime).Register(mux)
	videoapi.New(h.runtime).Register(mux)
	audioapi.New(h.runtime).Register(mux)
	whatsappapi.New(h.runtime).Register(mux)
	zettelapi.New(h.runtime).Register(mux)
}

func (h *Handler) cancelJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.runtime.CancelJob(r.Context(), r.PathValue("id"))
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, job)
}
