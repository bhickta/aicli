package documentapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
	"github.com/bhickta/aicli/internal/workflow/ocr"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/ocr/run", h.runOCR)
	mux.HandleFunc("POST /api/workflows/ocr/pdf", h.runPDFOCR)
	mux.HandleFunc("POST /api/workflows/analyze/run", h.runAnalyze)
}

func (h *Handler) runOCR(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		ocr.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "ocr", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("extracting images from ZIP", 2, 5)
		progress("OCR pages in parallel", 3, 5)
		result, err := ocr.New(p).Run(ctx, req.Request)
		progress("assembling markdown", 4, 5)
		return result, err
	})
}

func (h *Handler) runPDFOCR(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		ocr.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "pdf-ocr", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(fmt.Sprintf("rendering PDF pages with %d worker(s)", req.RenderWorkers), 2, 5)
		result, err := ocr.New(
			p,
			ocr.WithPDFRenderer(h.runtime.Settings().Tools, tool.ExecRunner{}),
		).RunPDFWithProgress(ctx, req.Request, func(stage string) {
			progress(stage, 3, 5)
		})
		return result, err
	})
}

func (h *Handler) runAnalyze(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		analyze.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "analyze", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("rendering and reading PDF", 2, 5)
		progress("analyzing OCR text", 3, 5)
		result, err := analyze.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).Run(ctx, req.Request)
		progress("saving analysis", 4, 5)
		return result, err
	})
}
