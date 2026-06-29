package documentapi

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
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
	mux.HandleFunc("GET /api/topper-reviews", h.listTopperReviews)
	mux.HandleFunc("GET /api/topper-reviews/{id}", h.getTopperReview)
	mux.HandleFunc("PUT /api/topper-reviews/{id}", h.updateTopperReview)
	mux.HandleFunc("DELETE /api/topper-reviews/{id}", h.deleteTopperReview)
	mux.HandleFunc("POST /api/topper-reviews/{id}/rerun", h.rerunTopperReview)
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
		progress(core.Indeterminate("extracting images from ZIP"))
		progress(core.Indeterminate("OCR pages in parallel"))
		result, err := ocr.New(p).Run(ctx, req.Request)
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
	job := core.NewJob("pdf-ocr", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		artifactDir := ""
		if h.runtime.DataDir() != "" {
			artifactDir = filepath.Join(h.runtime.DataDir(), "artifacts", "pdf-ocr", job.ID)
		}
		progress(core.Indeterminate(fmt.Sprintf("rendering PDF pages with %d worker(s)", req.RenderWorkers)))
		result, err := ocr.New(
			p,
			ocr.WithPDFRenderer(h.runtime.Settings().Tools, tool.ExecRunner{}),
			ocr.WithArtifactDir(artifactDir),
		).RunPDFWithProgress(ctx, req.Request, func(stage string) {
			progress(core.Indeterminate(stage))
		})
		if err == nil {
			_ = h.saveTopperReview(ctx, topperReviewFromOCR(job.ID, req.Path, result), topperReviewMeta{
				JobID:      job.ID,
				SourcePath: req.Path,
				ProviderID: req.ProviderID,
				Model:      req.Model,
				Status:     "ocr-only",
			})
		}
		return result, err
	})
}

func (h *Handler) runAnalyze(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID         string `json:"provider_id"`
		OCRProviderID      string `json:"ocr_provider_id"`
		QuestionProviderID string `json:"question_provider_id"`
		ReportProviderID   string `json:"report_provider_id"`
		analyze.Request
	}](w, r)
	if !ok {
		return
	}
	ocrProviderID := firstProviderID(req.OCRProviderID, req.ProviderID)
	questionProviderID := firstProviderID(req.QuestionProviderID, req.ProviderID, ocrProviderID)
	reportProviderID := firstProviderID(req.ReportProviderID, req.ProviderID, ocrProviderID)
	ocrProvider, ok := h.runtime.ProviderOrError(w, ocrProviderID)
	if !ok {
		return
	}
	questionProvider, ok := h.runtime.ProviderOrError(w, questionProviderID)
	if !ok {
		return
	}
	reportProvider, ok := h.runtime.ProviderOrError(w, reportProviderID)
	if !ok {
		return
	}
	job := core.NewJob("analyze", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		if req.UnloadModels {
			defer h.unloadProviderModels(context.Background(), []providerModelUse{
				{provider: ocrProvider, model: firstModel(req.OCRModel, req.Model)},
				{provider: questionProvider, model: firstModel(req.QuestionModel, req.Model)},
				{provider: reportProvider, model: firstModel(req.ReportModel, req.Model)},
			})
		}
		artifactDir := ""
		if h.runtime.DataDir() != "" {
			artifactDir = filepath.Join(h.runtime.DataDir(), "artifacts")
		}
		result, err := analyze.New(
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			ocrProvider,
			analyze.WithQuestionProvider(questionProvider),
			analyze.WithReportProvider(reportProvider),
			analyze.WithArtifactDir(artifactDir),
			analyze.WithLogger(h.runtime.Logger()),
		).RunWithProgress(ctx, req.Request, func(stage string, completed int, total int, label string) {
			progress(core.Units(stage, completed, total, label))
		})
		if err == nil {
			_ = h.saveTopperReview(ctx, result, topperReviewMeta{
				JobID:      job.ID,
				SourcePath: req.Path,
				ProviderID: ocrProviderID,
				Model:      firstModel(req.OCRModel, req.Model),
				Status:     "ready",
			})
		}
		return result, err
	})
}

type providerModelUse struct {
	provider provider.Provider
	model    string
}

func firstProviderID(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstModel(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) unloadProviderModels(ctx context.Context, uses []providerModelUse) {
	seen := map[string]bool{}
	for _, use := range uses {
		if use.provider == nil {
			continue
		}
		unloader, ok := use.provider.(provider.ModelUnloader)
		if !ok {
			continue
		}
		key := use.provider.ID() + "\x00" + strings.TrimSpace(use.model)
		if seen[key] {
			continue
		}
		seen[key] = true
		_ = unloader.UnloadModel(ctx, use.model)
	}
}
