package documentapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
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
	ocrProviderID := strings.TrimSpace(req.OCRProviderID)
	questionProviderID := strings.TrimSpace(req.QuestionProviderID)
	reportProviderID := strings.TrimSpace(req.ReportProviderID)
	ocrModel := strings.TrimSpace(req.OCRModel)
	questionModel := strings.TrimSpace(req.QuestionModel)
	reportModel := strings.TrimSpace(req.ReportModel)
	directPDFRequested := shouldUseDirectPDFMode(ocrProviderID, req.OCRInputMode)
	preferredReviewID := strings.TrimSpace(req.ReviewID)
	cachedReview, cachedOutput, cachedFound, err := h.findReusableTopperReview(
		r.Context(),
		req.Path,
		directPDFRequested,
		preferredReviewID,
	)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	useCachedDirectPDF := cachedFound && !req.ForceOCR && directPDFRequested && cachedOutput.SourceMode == analyze.OCRInputModePDFDirect
	useCachedOCR := cachedFound && !req.ForceOCR && !directPDFRequested
	useDirectPDF := directPDFRequested && !useCachedDirectPDF
	if useCachedDirectPDF {
		job := core.NewJob("analyze", req.Path)
		h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
			progress(core.Units("using saved Gemini-Lite PDF review", 1, 1, "step"))
			if preferredReviewID != "" && cachedOutput.ReviewID != preferredReviewID {
				cachedOutput.ReviewID = preferredReviewID
				if store, ok := h.runtime.Store().(topperReviewStore); ok {
					err := h.saveTopperReviewRecord(ctx, store, cachedOutput, topperReviewMeta{
						JobID:      job.ID,
						SourcePath: req.Path,
						ProviderID: firstProviderID(ocrProviderID, cachedReview.ProviderID),
						Model:      firstModel(ocrModel, cachedReview.Model),
						Status:     cachedReview.Status,
					}, cachedReview.CreatedAt)
					if err != nil {
						return nil, err
					}
				}
			}
			return cachedOutput, nil
		})
		return
	}
	if err := requireAnalyzeStageSelections(!useCachedOCR, req.QuestionSplit && !useDirectPDF, ocrProviderID, ocrModel, questionProviderID, questionModel, reportProviderID, reportModel, !useDirectPDF); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var ocrProvider provider.Provider
	if !useCachedOCR {
		ocrProvider, ok = h.runtime.ProviderOrError(w, ocrProviderID)
		if !ok {
			return
		}
	}
	questionProvider := ocrProvider
	reportProvider := ocrProvider
	if !useDirectPDF {
		questionProvider, ok = h.runtime.ProviderOrError(w, questionProviderID)
		if !ok {
			return
		}
		reportProvider, ok = h.runtime.ProviderOrError(w, reportProviderID)
		if !ok {
			return
		}
	}
	job := core.NewJob("analyze", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		if req.UnloadModels {
			defer h.unloadProviderModels(context.Background(), []providerModelUse{
				{provider: ocrProvider, model: ocrModel},
				{provider: questionProvider, model: questionModel},
				{provider: reportProvider, model: reportModel},
			})
		}
		artifactDir := ""
		if h.runtime.DataDir() != "" {
			artifactDir = filepath.Join(h.runtime.DataDir(), "artifacts")
		}
		request := analyze.Request{
			OCRModel:        ocrModel,
			QuestionModel:   questionModel,
			ReportModel:     reportModel,
			Path:            req.Path,
			DPI:             req.DPI,
			RenderWorkers:   req.RenderWorkers,
			Workers:         req.Workers,
			OCRBatchSize:    req.OCRBatchSize,
			OCRInputMode:    req.OCRInputMode,
			QuestionSplit:   req.QuestionSplit,
			QuestionWorkers: req.QuestionWorkers,
			UnloadModels:    req.UnloadModels,
			ForceOCR:        req.ForceOCR,
			ReviewID:        preferredReviewID,
		}
		createdAt := time.Time{}
		if useCachedOCR {
			request.ReviewID = firstProviderID(preferredReviewID, cachedOutput.ReviewID)
			request.OCRPages = cachedOutput.Pages
			createdAt = cachedReview.CreatedAt
		}
		store, storeOK := h.runtime.Store().(topperReviewStore)
		result, err := analyze.New(
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			ocrProvider,
			analyze.WithQuestionProvider(questionProvider),
			analyze.WithReportProvider(reportProvider),
			analyze.WithArtifactDir(artifactDir),
			analyze.WithLogger(h.runtime.Logger()),
			analyze.WithOCRCheckpoint(func(review analyze.Response) error {
				if !storeOK {
					return nil
				}
				return h.saveTopperReviewRecord(ctx, store, review, topperReviewMeta{
					JobID:      job.ID,
					SourcePath: req.Path,
					ProviderID: ocrProviderID,
					Model:      ocrModel,
					Status:     "ocr-ready",
				}, createdAt)
			}),
		).RunWithProgress(ctx, request, func(stage string, completed int, total int, label string) {
			progress(core.Units(stage, completed, total, label))
		})
		if err == nil && storeOK {
			_ = h.saveTopperReviewRecord(ctx, store, result, topperReviewMeta{
				JobID:      job.ID,
				SourcePath: req.Path,
				ProviderID: ocrProviderID,
				Model:      ocrModel,
				Status:     "ready",
			}, createdAt)
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

func (h *Handler) findReusableTopperReview(ctx context.Context, sourcePath string, directPDF bool, preferredID string) (storage.TopperReviewRecord, analyze.Response, bool, error) {
	store, ok := h.runtime.Store().(topperReviewStore)
	if !ok {
		return storage.TopperReviewRecord{}, analyze.Response{}, false, nil
	}
	if err := h.backfillTopperReviews(ctx, store); err != nil {
		return storage.TopperReviewRecord{}, analyze.Response{}, false, err
	}
	preferredID = strings.TrimSpace(preferredID)
	if preferredID != "" {
		record, err := store.GetTopperReview(ctx, preferredID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return storage.TopperReviewRecord{}, analyze.Response{}, false, err
		}
		if err == nil {
			review, err := decodeTopperReview(record)
			if err != nil {
				return storage.TopperReviewRecord{}, analyze.Response{}, false, err
			}
			if topperReviewMatchesMode(review, record, directPDF) {
				return record, review, true, nil
			}
		}
	}
	filename := strings.ToLower(filepath.Base(sourcePath))
	records, err := store.ListTopperReviews(ctx, storage.TopperReviewListOptions{Query: filename, Limit: 200})
	if err != nil {
		return storage.TopperReviewRecord{}, analyze.Response{}, false, err
	}
	sourcePathLower := strings.ToLower(strings.TrimSpace(sourcePath))
	for _, summary := range records {
		recordSource := strings.ToLower(strings.TrimSpace(summary.SourcePath))
		recordPDFName := strings.ToLower(summary.PDFName)
		if sourcePathLower != "" && recordSource != sourcePathLower && recordPDFName != filename {
			continue
		}
		record, err := store.GetTopperReview(ctx, summary.ID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				continue
			}
			return storage.TopperReviewRecord{}, analyze.Response{}, false, err
		}
		review, err := decodeTopperReview(record)
		if err != nil {
			continue
		}
		if topperReviewMatchesMode(review, record, directPDF) {
			return record, review, true, nil
		}
	}
	return storage.TopperReviewRecord{}, analyze.Response{}, false, nil
}

func topperReviewMatchesMode(review analyze.Response, record storage.TopperReviewRecord, directPDF bool) bool {
	if directPDF {
		return review.SourceMode == analyze.OCRInputModePDFDirect && strings.EqualFold(record.Status, "ready")
	}
	return reviewHasOCRPages(review)
}

func reviewHasOCRPages(review analyze.Response) bool {
	if len(review.Pages) == 0 {
		return false
	}
	for _, page := range review.Pages {
		if strings.TrimSpace(page.Text) != "" {
			return true
		}
	}
	return false
}

func shouldUseDirectPDFMode(providerID string, mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "pdf_direct" {
		return true
	}
	return (mode == "" || mode == "auto") && strings.Contains(strings.ToLower(strings.TrimSpace(providerID)), "gemini")
}

func requireAnalyzeStageSelections(requireOCR bool, questionSplit bool, ocrProviderID string, ocrModel string, questionProviderID string, questionModel string, reportProviderID string, reportModel string, requireReport bool) error {
	if requireOCR {
		if ocrProviderID == "" {
			return fmt.Errorf("OCR provider is required")
		}
		if ocrModel == "" {
			return fmt.Errorf("OCR model is required")
		}
	}
	if questionSplit {
		if questionProviderID == "" {
			return fmt.Errorf("question split provider is required")
		}
		if questionModel == "" {
			return fmt.Errorf("question split model is required")
		}
	}
	if requireReport {
		if reportProviderID == "" {
			return fmt.Errorf("report provider is required")
		}
		if reportModel == "" {
			return fmt.Errorf("report model is required")
		}
	}
	return nil
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
