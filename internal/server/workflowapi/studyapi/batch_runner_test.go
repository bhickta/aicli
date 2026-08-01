package studyapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

type studyModelProvider struct {
	models []provider.Model
	err    error
}

type studyLoadedModelProvider struct {
	*studyModelProvider
	loaded []provider.Model
}

func (p *studyLoadedModelProvider) ListLoadedModels(context.Context) ([]provider.Model, error) {
	return p.loaded, p.err
}

func (p *studyModelProvider) ID() string                   { return "lms" }
func (p *studyModelProvider) Health(context.Context) error { return p.err }
func (p *studyModelProvider) ListModels(context.Context) ([]provider.Model, error) {
	return p.models, p.err
}
func (p *studyModelProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("not implemented")
}
func (p *studyModelProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("not implemented")
}
func (p *studyModelProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("not implemented")
}

func TestResolveStudyBatchRunOptionsUsesLoadedLMStudioModel(t *testing.T) {
	t.Parallel()

	p := &studyModelProvider{models: []provider.Model{{ID: "qwen-vision-loaded"}}}
	h := &Handler{runtime: core.New(core.Dependencies{
		Settings: func() config.Settings { return config.Settings{} },
		ProviderFor: func(id string) (provider.Provider, bool) {
			return p, id == "lms"
		},
	})}

	options, gotProvider, err := h.resolveStudyBatchRunOptions(context.Background(), studyBatchRunOptions{})
	if err != nil {
		t.Fatalf("resolveStudyBatchRunOptions() error = %v", err)
	}
	if gotProvider != p || options.ProviderID != "lms" || options.Model != "qwen-vision-loaded" || options.Parallelism != 1 {
		t.Fatalf("options = %#v, provider = %#v, want local loaded model and conservative parallelism", options, gotProvider)
	}
	if options.OCRModel != options.Model ||
		options.QuestionModel != options.Model ||
		options.BoundaryModel != options.Model ||
		options.ReportModel != options.Model {
		t.Fatalf("stage models = %#v, want discovered fallback applied consistently", options)
	}
}

func TestResolveStudyBatchRunOptionsPreservesStageSpecificModels(t *testing.T) {
	t.Parallel()

	p := &studyModelProvider{}
	h := &Handler{runtime: core.New(core.Dependencies{
		Settings: func() config.Settings { return config.Settings{} },
		ProviderFor: func(id string) (provider.Provider, bool) {
			return p, id == "lms"
		},
	})}

	options, _, err := h.resolveStudyBatchRunOptions(context.Background(), studyBatchRunOptions{
		OCRModel:      "unlimited-ocr",
		QuestionModel: "qwen/qwen3.6-27b",
		BoundaryModel: "prismml/bonsai-27b",
		ReportModel:   "qwen/qwen3.6-27b",
	})
	if err != nil {
		t.Fatalf("resolveStudyBatchRunOptions() error = %v", err)
	}
	if options.OCRModel != "unlimited-ocr" ||
		options.QuestionModel != "qwen/qwen3.6-27b" ||
		options.BoundaryModel != "prismml/bonsai-27b" ||
		options.ReportModel != "qwen/qwen3.6-27b" {
		t.Fatalf("options = %#v, want stage-specific local models preserved", options)
	}
	if options.Model != "qwen/qwen3.6-27b" {
		t.Fatalf("batch model = %q, want primary analysis model", options.Model)
	}
}

func TestResolveStudyBatchRunOptionsRejectsUnloadedAnalysisModel(t *testing.T) {
	t.Parallel()

	p := &studyLoadedModelProvider{
		studyModelProvider: &studyModelProvider{},
		loaded:             []provider.Model{{ID: "google/gemma-4-e2b"}},
	}
	h := &Handler{runtime: core.New(core.Dependencies{
		Settings: func() config.Settings { return config.Settings{} },
		ProviderFor: func(id string) (provider.Provider, bool) {
			return p, id == "lms"
		},
	})}

	_, _, err := h.resolveStudyBatchRunOptions(context.Background(), studyBatchRunOptions{
		QuestionModel: "google/gemma-4-e2b",
		BoundaryModel: "qwen/qwen3.6-27b",
		ReportModel:   "google/gemma-4-e2b",
	})
	if err == nil || !strings.Contains(err.Error(), "not loaded or unavailable") || !strings.Contains(err.Error(), "qwen/qwen3.6-27b") {
		t.Fatalf("error = %v, want actionable unloaded boundary-model error", err)
	}
}

func TestResolveStudyBatchRunOptionsDiscoversActuallyLoadedModel(t *testing.T) {
	t.Parallel()

	p := &studyLoadedModelProvider{
		studyModelProvider: &studyModelProvider{
			models: []provider.Model{{ID: "downloaded-but-unloaded"}, {ID: "qwen/qwen3.6-27b"}},
		},
		loaded: []provider.Model{{ID: "qwen/qwen3.6-27b"}},
	}
	h := &Handler{runtime: core.New(core.Dependencies{
		Settings: func() config.Settings { return config.Settings{} },
		ProviderFor: func(id string) (provider.Provider, bool) {
			return p, id == "lms"
		},
	})}

	options, _, err := h.resolveStudyBatchRunOptions(context.Background(), studyBatchRunOptions{})
	if err != nil {
		t.Fatalf("resolveStudyBatchRunOptions() error = %v", err)
	}
	if options.Model != "qwen/qwen3.6-27b" || options.QuestionModel != "qwen/qwen3.6-27b" {
		t.Fatalf("options = %#v, want actually loaded model selected", options)
	}
}

func TestResolveStudyBatchRunOptionsExplainsMissingLMStudioModel(t *testing.T) {
	t.Parallel()

	p := &studyModelProvider{}
	h := &Handler{runtime: core.New(core.Dependencies{
		Settings: func() config.Settings { return config.Settings{} },
		ProviderFor: func(id string) (provider.Provider, bool) {
			return p, id == "lms"
		},
	})}

	_, _, err := h.resolveStudyBatchRunOptions(context.Background(), studyBatchRunOptions{})
	if err == nil || !strings.Contains(err.Error(), "no loaded models") || !strings.Contains(err.Error(), "vision-capable") {
		t.Fatalf("error = %v, want actionable LM Studio model-loading guidance", err)
	}
}

func TestLoadStudyOCRCheckpointResumesOnlyMatchingOCRReadyRecord(t *testing.T) {
	t.Parallel()

	store := newStudySyncTestStore(t)
	review := analyze.Response{
		ReviewID:   "copy-1",
		SourceMode: analyze.OCRInputModeImages,
		Pages:      []analyze.Page{{Number: 1, Text: "saved OCR"}},
	}
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTopperReview(context.Background(), storage.TopperReviewRecord{
		ID:         "copy-1",
		SourcePath: "/tmp/copy.pdf",
		Status:     "ocr_ready",
		ReviewJSON: string(data),
	}); err != nil {
		t.Fatal(err)
	}

	pages, err := loadStudyOCRCheckpoint(context.Background(), store, storage.StudyCopyRecord{
		ID: "copy-1", SourcePath: "/tmp/copy.pdf",
	}, false)
	if err != nil || len(pages) != 1 || pages[0].Text != "saved OCR" {
		t.Fatalf("pages = %#v, error = %v, want saved OCR resume", pages, err)
	}
	record, err := store.GetTopperReview(context.Background(), "copy-1")
	if err != nil {
		t.Fatal(err)
	}
	record.Status = "needs_review"
	if err := store.SaveTopperReview(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	pages, err = loadStudyOCRCheckpoint(context.Background(), store, storage.StudyCopyRecord{
		ID: "copy-1", SourcePath: "/tmp/copy.pdf",
	}, false)
	if err != nil || len(pages) != 1 {
		t.Fatalf("needs-review pages = %#v, error = %v, want OCR reused for refinement", pages, err)
	}
	pages, err = loadStudyOCRCheckpoint(context.Background(), store, storage.StudyCopyRecord{
		ID: "copy-1", SourcePath: "/tmp/different.pdf",
	}, false)
	if err != nil || len(pages) != 0 {
		t.Fatalf("mismatched pages = %#v, error = %v, want checkpoint ignored", pages, err)
	}
	pages, err = loadStudyOCRCheckpoint(context.Background(), store, storage.StudyCopyRecord{
		ID: "copy-1", SourcePath: "/tmp/copy.pdf",
	}, true)
	if err != nil || len(pages) != 0 {
		t.Fatalf("forced pages = %#v, error = %v, want checkpoint bypassed", pages, err)
	}
}

func TestStudyTopperReviewStatusRequiresReviewWhenQualityWarns(t *testing.T) {
	t.Parallel()

	if got := studyTopperReviewStatus(analyze.Response{}); got != "ready" {
		t.Fatalf("status without quality = %q, want backward-compatible ready", got)
	}
	if got := studyTopperReviewStatus(analyze.Response{Quality: &analyze.AnalysisQuality{RequiresReview: true}}); got != "needs_review" {
		t.Fatalf("review status = %q, want needs_review for failed quality gates", got)
	}
}

func TestStudyBatchWorkerAllocationSharesOneBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		parallelism      int
		copies           int
		wantCopyWorkers  int
		wantModelWorkers int
	}{
		{name: "defaults invalid input", parallelism: 0, copies: 0, wantCopyWorkers: 1, wantModelWorkers: 1},
		{name: "single copy gets every slot", parallelism: 4, copies: 1, wantCopyWorkers: 1, wantModelWorkers: 4},
		{name: "copies consume the budget first", parallelism: 4, copies: 4, wantCopyWorkers: 4, wantModelWorkers: 1},
		{name: "remaining capacity is divided safely", parallelism: 5, copies: 2, wantCopyWorkers: 2, wantModelWorkers: 2},
		{name: "copy workers are capped by selected copies", parallelism: 5, copies: 3, wantCopyWorkers: 3, wantModelWorkers: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyWorkers, modelWorkers := studyBatchWorkerAllocation(tt.parallelism, tt.copies)
			if copyWorkers != tt.wantCopyWorkers || modelWorkers != tt.wantModelWorkers {
				t.Fatalf(
					"studyBatchWorkerAllocation(%d, %d) = (%d, %d), want (%d, %d)",
					tt.parallelism,
					tt.copies,
					copyWorkers,
					modelWorkers,
					tt.wantCopyWorkers,
					tt.wantModelWorkers,
				)
			}
			budget := tt.parallelism
			if budget < 1 {
				budget = 1
			}
			if tt.copies > 0 && copyWorkers*modelWorkers > budget {
				t.Fatalf("allocation exceeds shared parallelism budget")
			}
		})
	}
}

func TestStudyAnalyzeRequestsUseAssignedModelWorkers(t *testing.T) {
	t.Parallel()

	copyRecord := storage.StudyCopyRecord{ID: "copy-1", SourcePath: "/tmp/copy.pdf"}
	options := studyBatchRunOptions{
		Model:         "analysis-model",
		OCRModel:      "ocr-model",
		QuestionModel: "question-model",
		BoundaryModel: "boundary-model",
		ReportModel:   "report-model",
		ModelWorkers:  3,
		ForceOCR:      true,
	}
	ocrPages := []analyze.Page{{Number: 1, Text: "saved OCR"}}

	analysisReq := newStudyAnalysisRequest(copyRecord, options, ocrPages)
	if analysisReq.RenderWorkers != 3 || analysisReq.Workers != 3 || analysisReq.QuestionWorkers != 3 {
		t.Fatalf("analysis workers = render:%d OCR:%d question:%d, want 3 for each", analysisReq.RenderWorkers, analysisReq.Workers, analysisReq.QuestionWorkers)
	}
	if analysisReq.ForceOCR {
		t.Fatal("analysis request forced OCR despite supplied checkpoint")
	}
	if len(analysisReq.OCRPages) != 1 || analysisReq.OCRPages[0].Text != "saved OCR" {
		t.Fatalf("analysis OCR pages = %#v, want supplied checkpoint", analysisReq.OCRPages)
	}

	ocrReq := newStudyOCRRequest(copyRecord, options)
	if ocrReq.Model != "ocr-model" || ocrReq.Workers != 3 || ocrReq.RenderWorkers != 3 {
		t.Fatalf("OCR request = %#v, want OCR model with three workers", ocrReq)
	}
	if !ocrReq.ForceOCR {
		t.Fatal("OCR-only request did not preserve force OCR")
	}
}
