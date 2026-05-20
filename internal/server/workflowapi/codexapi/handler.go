package codexapi

import (
	"context"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/codex"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/codex/run", h.runCodex)
	mux.HandleFunc("POST /api/workflows/codex/cli", h.runCodexCLI)
}

func (h *Handler) runCodex(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		codex.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}

	h.runtime.StartJob(w, r, "codex", req.Task, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("running Codex coding workflow", 2, 4)
		result, err := codex.New(p).Run(ctx, req.Request)
		progress("saving Codex response", 3, 4)
		return result, err
	})
}

func (h *Handler) runCodexCLI(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[codex.CLIRequest](w, r)
	if !ok {
		return
	}

	h.runtime.StartJob(w, r, "codex-cli", req.Task, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("running Codex CLI with local ChatGPT auth", 2, 4)
		result, err := codex.NewCLI(h.runtime.Settings().Tools, tool.ExecRunner{}).Run(ctx, req)
		progress("saving Codex CLI response", 3, 4)
		return result, err
	})
}
