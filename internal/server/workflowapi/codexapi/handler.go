package codexapi

import (
	"context"
	"encoding/json"
	"errors"
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
	var req struct {
		ProviderID string `json:"provider_id"`
		codex.Request
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

	job := core.NewJob("codex", req.Task)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("running Codex coding workflow", 2, 4)
		result, err := codex.New(p).Run(ctx, req.Request)
		progress("saving Codex response", 3, 4)
		return result, err
	})
}

func (h *Handler) runCodexCLI(w http.ResponseWriter, r *http.Request) {
	var req codex.CLIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}

	job := core.NewJob("codex-cli", req.Task)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("running Codex CLI with local ChatGPT auth", 2, 4)
		result, err := codex.NewCLI(h.runtime.Settings().Tools, tool.ExecRunner{}).Run(ctx, req)
		progress("saving Codex CLI response", 3, 4)
		return result, err
	})
}
