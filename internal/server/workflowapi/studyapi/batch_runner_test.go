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
