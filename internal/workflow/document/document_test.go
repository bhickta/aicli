package document

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct{}

func (fakeRunner) CombinedOutput(_ context.Context, _ string, args ...string) ([]byte, error) {
	prefix := args[len(args)-1]
	return []byte("ok"), os.WriteFile(prefix+"-1.jpg", []byte("image"), 0o600)
}

type pageRunner struct {
	mu            sync.Mutex
	active        int
	maxActive     int
	renderedPages int
}

func (r *pageRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	if strings.HasSuffix(command, "pdfinfo") {
		return []byte("Pages: 6\n"), nil
	}

	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()

	prefix := args[len(args)-1]
	err := os.WriteFile(prefix+"-1.jpg", []byte("image"), 0o600)

	r.mu.Lock()
	r.active--
	r.renderedPages++
	r.mu.Unlock()
	return []byte("ok"), err
}

type missingPDFInfoRunner struct {
	rendered bool
}

func (r *missingPDFInfoRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	if strings.HasSuffix(command, "pdfinfo") {
		return nil, errors.New("exit status 1")
	}
	r.rendered = true
	prefix := args[len(args)-1]
	return []byte("ok"), os.WriteFile(prefix+"-1.jpg", []byte("image"), 0o600)
}

type fakeVision struct{}

func (fakeVision) ID() string { return "fake" }
func (fakeVision) Health(context.Context) error {
	return nil
}
func (fakeVision) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (fakeVision) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (fakeVision) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (fakeVision) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: "page text"}, nil
}

type flakyVision struct{}

func (flakyVision) ID() string { return "flaky" }
func (flakyVision) Health(context.Context) error {
	return nil
}
func (flakyVision) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (flakyVision) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (flakyVision) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (flakyVision) Vision(_ context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	if string(req.Image) == "fail" {
		return provider.ChatResponse{}, errors.New("server overloaded")
	}
	return provider.ChatResponse{Content: "ok"}, nil
}

type badOCRVision struct {
	response provider.ChatResponse
}

func (badOCRVision) ID() string { return "bad-ocr" }
func (badOCRVision) Health(context.Context) error {
	return nil
}
func (badOCRVision) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (badOCRVision) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (badOCRVision) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (v badOCRVision) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return v.response, nil
}

func TestRenderPDFToImages(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	images, cleanup, err := RenderPDFToImages(context.Background(), config.ToolConfig{PDFToPPM: "pdftoppm"}, fakeRunner{}, pdf, 200, 1)
	if err != nil {
		t.Fatalf("RenderPDFToImages() error = %v", err)
	}
	defer cleanup()
	if len(images) != 1 {
		t.Fatalf("images = %#v, want one image", images)
	}
}

func TestRenderPDFToImagesRendersPagesWithDynamicParallelism(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &pageRunner{}
	images, cleanup, err := RenderPDFToImages(context.Background(), config.ToolConfig{PDFToPPM: "pdftoppm"}, runner, pdf, 200, 3)
	if err != nil {
		t.Fatalf("RenderPDFToImages() error = %v", err)
	}
	defer cleanup()
	if len(images) != 6 {
		t.Fatalf("images = %#v, want six images", images)
	}
	if runner.renderedPages != 6 {
		t.Fatalf("renderedPages = %d, want 6", runner.renderedPages)
	}
	if runner.maxActive > 3 {
		t.Fatalf("maxActive = %d, want <= 3", runner.maxActive)
	}
}

func TestRenderPDFToImagesFallsBackWhenPDFInfoUnavailable(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &missingPDFInfoRunner{}
	images, cleanup, err := RenderPDFToImages(context.Background(), config.ToolConfig{PDFToPPM: "pdftoppm"}, runner, pdf, 200, 4)
	if err != nil {
		t.Fatalf("RenderPDFToImages() error = %v", err)
	}
	defer cleanup()
	if !runner.rendered {
		t.Fatal("RenderPDFToImages() did not fall back to normal rendering")
	}
	if len(images) != 1 {
		t.Fatalf("images = %#v, want one image", images)
	}
}

func TestOCRImagesKeepsInputOrder(t *testing.T) {
	t.Parallel()

	pages, err := OCRImages(
		context.Background(),
		fakeVision{},
		"model",
		[]ImageInput{{Name: "b", Data: []byte("2")}, {Name: "a", Data: []byte("1")}},
		"prompt",
		2,
		nil,
	)
	if err != nil {
		t.Fatalf("OCRImages() error = %v", err)
	}
	if pages[0].Name != "b" || pages[1].Name != "a" {
		t.Fatalf("pages = %#v, want input order preserved", pages)
	}
}

func TestOCRImagesKeepsPartialPageFailures(t *testing.T) {
	t.Parallel()

	pages, err := OCRImages(
		context.Background(),
		flakyVision{},
		"model",
		[]ImageInput{{Name: "page-1", Data: []byte("ok")}, {Name: "page-2", Data: []byte("fail")}},
		"prompt",
		0,
		nil,
	)
	if err != nil {
		t.Fatalf("OCRImages() error = %v", err)
	}
	if pages[0].Text != "ok" {
		t.Fatalf("page 1 text = %q, want ok", pages[0].Text)
	}
	if !strings.Contains(pages[1].Text, "server overloaded") {
		t.Fatalf("page 2 text = %q, want failure marker", pages[1].Text)
	}
}

func TestOCRImagesRejectsTruncatedAndServerErrorResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response provider.ChatResponse
		want     string
	}{
		{
			name:     "truncated",
			response: provider.ChatResponse{Content: "partial repeated OCR", FinishReason: "length"},
			want:     "truncated",
		},
		{
			name:     "html error page",
			response: provider.ChatResponse{Content: "<html><body><pre>Internal Server Error</pre></body></html>"},
			want:     "error page",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages, err := OCRImages(
				context.Background(),
				badOCRVision{response: tt.response},
				"model",
				[]ImageInput{{Name: "page-1", Data: []byte("image")}},
				"prompt",
				1,
				nil,
			)
			if err == nil {
				t.Fatal("OCRImages() error = nil, want all-pages failure")
			}
			if len(pages) != 1 || !strings.Contains(pages[0].Text, tt.want) {
				t.Fatalf("pages = %#v, want failure marker containing %q", pages, tt.want)
			}
		})
	}
}

func TestCleanOCRResponseStripsDetectorTagsAndDeduplicates(t *testing.T) {
	t.Parallel()

	got, err := cleanOCRResponse(provider.ChatResponse{Content: strings.Join([]string{
		"<|det|>title [382, 12, 639, 56]<|/det|>ForumIAS",
		"<|det|>text [438, 291, 884, 513]<|/det|>of words but a number of",
		"<|det|>text [111, 292, 885, 512]<|/det|>of words but a number of",
		"<|det|>text [504, 280, 854, 308]<|/det|>short",
		"<|det|>text [516, 318, 851, 345]<|/det|>short",
	}, "\n")})
	if err != nil {
		t.Fatalf("cleanOCRResponse() error = %v", err)
	}
	if strings.Contains(got, "<|det|>") || strings.Contains(got, "[382, 12, 639, 56]") {
		t.Fatalf("cleaned OCR = %q, still contains detector tags", got)
	}
	if strings.Count(got, "of words but a number of") != 1 {
		t.Fatalf("cleaned OCR = %q, want long duplicate collapsed", got)
	}
	if strings.Count(got, "short") != 2 {
		t.Fatalf("cleaned OCR = %q, want short repeated text preserved", got)
	}
}

func TestOCRImagesReportsProgress(t *testing.T) {
	t.Parallel()

	updates := []int{}
	_, err := OCRImages(
		context.Background(),
		fakeVision{},
		"model",
		[]ImageInput{{Name: "p1", Data: []byte("1")}, {Name: "p2", Data: []byte("2")}},
		"prompt",
		1,
		func(completed int, total int) {
			if total != 2 {
				t.Fatalf("total = %d, want 2", total)
			}
			updates = append(updates, completed)
		},
	)
	if err != nil {
		t.Fatalf("OCRImages() error = %v", err)
	}
	if len(updates) != 2 || updates[0] != 1 || updates[1] != 2 {
		t.Fatalf("updates = %#v, want [1 2]", updates)
	}
}

func TestEffectiveOCRWorkersHonorsExplicitWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		workers int
		jobs    int
		want    int
	}{
		{name: "uses explicit worker count", workers: 48, jobs: 48, want: 48},
		{name: "caps explicit workers by available jobs", workers: 48, jobs: 8, want: 8},
		{name: "single job needs one worker", workers: 48, jobs: 1, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveOCRWorkers(tt.workers, tt.jobs); got != tt.want {
				t.Fatalf("EffectiveOCRWorkers(%d, %d) = %d, want %d", tt.workers, tt.jobs, got, tt.want)
			}
		})
	}
}
