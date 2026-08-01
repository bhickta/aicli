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
	if options.OCRModel != options.Model || options.QuestionModel != options.Model || options.ReportModel != options.Model {
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
		ReportModel:   "qwen/qwen3.6-27b",
	})
	if err != nil {
		t.Fatalf("resolveStudyBatchRunOptions() error = %v", err)
	}
	if options.OCRModel != "unlimited-ocr" || options.QuestionModel != "qwen/qwen3.6-27b" || options.ReportModel != "qwen/qwen3.6-27b" {
		t.Fatalf("options = %#v, want stage-specific local models preserved", options)
	}
	if options.Model != "qwen/qwen3.6-27b" {
		t.Fatalf("batch model = %q, want primary analysis model", options.Model)
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
