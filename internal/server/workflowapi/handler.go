package workflowapi

import (
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/audioapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/server/workflowapi/documentapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/imageapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/newsapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/studyapi"
	"github.com/bhickta/aicli/internal/server/workflowapi/videoapi"
)

type Dependencies = core.Dependencies

type Handler struct {
	runtime *core.Runtime
}

func New(deps Dependencies) *Handler {
	return &Handler{runtime: core.New(deps)}
}

func (h *Handler) Register(mux *http.ServeMux) {
	studyapi.New(h.runtime).Register(mux)
	imageapi.New(h.runtime).Register(mux)
	newsapi.New(h.runtime).Register(mux)
	documentapi.New(h.runtime).Register(mux)
	videoapi.New(h.runtime).Register(mux)
	audioapi.New(h.runtime).Register(mux)
}
