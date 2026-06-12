package geminiapi

import (
	"context"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/gemini"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/gemini/run", h.runGemini)
}

func (h *Handler) runGemini(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[gemini.Request](w, r)
	if !ok {
		return
	}

	h.runtime.StartJob(w, r, "gemini-cli", req.Task, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("running Gemini CLI autonomous workflow"))
		result, err := gemini.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Run(ctx, req)
		return result, err
	})
}
