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

type localFakeVision struct {
	fakeVision
}

func (localFakeVision) ID() string { return "custom-local" }
func (localFakeVision) LocalModelServer() bool {
	return true
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

type retryVision struct {
	mu   sync.Mutex
	seen map[string]int
}

func (retryVision) ID() string { return "retry" }
func (retryVision) Health(context.Context) error {
	return nil
}
func (retryVision) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (retryVision) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (retryVision) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (v *retryVision) Vision(_ context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	key := string(req.Image)
	v.mu.Lock()
	if v.seen == nil {
		v.seen = map[string]int{}
	}
	v.seen[key]++
	count := v.seen[key]
	v.mu.Unlock()
	if count == 1 {
		return provider.ChatResponse{}, errors.New("temporary server overload")
	}
	return provider.ChatResponse{Content: key + " recovered"}, nil
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

type unsupportedImageVision struct {
	fakeVision
}

func (unsupportedImageVision) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New(`chat: 400 Bad Request: {"error":"Model does not support images. Please use a model that does."}`)
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

func TestOCRImagesRetriesFailedPagesSerially(t *testing.T) {
	t.Parallel()

	vision := &retryVision{}
	pages, err := OCRImages(
		context.Background(),
		vision,
		"model",
		[]ImageInput{{Name: "page-1", Data: []byte("one")}, {Name: "page-2", Data: []byte("two")}},
		"prompt",
		2,
		nil,
	)
	if err != nil {
		t.Fatalf("OCRImages() error = %v", err)
	}
	if pages[0].Text != "one recovered" || pages[1].Text != "two recovered" {
		t.Fatalf("pages = %#v, want recovered OCR text after retry", pages)
	}
}

func TestOCRImagesFailsFastForTextOnlyModel(t *testing.T) {
	t.Parallel()

	_, err := OCRImages(
		context.Background(),
		unsupportedImageVision{},
		"text-model",
		[]ImageInput{{Name: "page-1", Data: []byte("image")}, {Name: "page-2", Data: []byte("image")}},
		"prompt",
		2,
		nil,
	)
	if err == nil {
		t.Fatal("OCRImages() error = nil, want text-only model error")
	}
	for _, want := range []string{"text-model", "does not support images", "unlimited-ocr", "OCR model field"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestOCRImagesKeepsTruncatedTextAndRejectsServerErrorResponses(t *testing.T) {
	t.Parallel()

	got, err := cleanOCRResponse(provider.ChatResponse{Content: "partial OCR", FinishReason: "length"})
	if err != nil {
		t.Fatalf("cleanOCRResponse(truncated) error = %v", err)
	}
	if !strings.Contains(got, "partial OCR") || !strings.Contains(got, "OCR truncated") {
		t.Fatalf("cleaned truncated OCR = %q, want partial text and marker", got)
	}

	pages, err := OCRImages(
		context.Background(),
		badOCRVision{response: provider.ChatResponse{Content: "<html><body><pre>Internal Server Error</pre></body></html>"}},
		"model",
		[]ImageInput{{Name: "page-1", Data: []byte("image")}},
		"prompt",
		1,
		nil,
	)
	if err == nil {
		t.Fatal("OCRImages(server error) error = nil, want all-pages failure")
	}
	if len(pages) != 1 || !strings.Contains(pages[0].Text, "error page") {
		t.Fatalf("pages = %#v, want failure marker containing error page", pages)
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
	if got, err := cleanOCRResponse(provider.ChatResponse{Content: `$\\rightarrow$ line`}); err != nil || strings.Contains(got, `\rightarrow`) {
		t.Fatalf("cleaned OCR = %q, err = %v, want arrow normalized", got, err)
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

func TestEffectiveOCRWorkersCapsLocalModelServers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		providerID string
		workers    int
		jobs       int
		want       int
	}{
		{name: "lm studio auto", providerID: "lms", workers: 0, jobs: 48, want: 1},
		{name: "lm studio explicit", providerID: "lms", workers: 48, jobs: 48, want: 48},
		{name: "ollama explicit", providerID: "ollama", workers: 8, jobs: 48, want: 8},
		{name: "vllm explicit", providerID: "vllm", workers: 8, jobs: 48, want: 8},
		{name: "remote explicit", providerID: "openrouter", workers: 8, jobs: 48, want: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveOCRWorkersForProvider(tt.workers, tt.jobs, tt.providerID)
			if got != tt.want {
				t.Fatalf("EffectiveOCRWorkersForProvider(%d, %d, %q) = %d, want %d", tt.workers, tt.jobs, tt.providerID, got, tt.want)
			}
		})
	}
}

func TestEffectiveOCRWorkersUsesProviderCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		vision  provider.Provider
		workers int
		jobs    int
		want    int
	}{
		{name: "custom local provider caps auto workers", vision: localFakeVision{}, workers: 0, jobs: 48, want: 1},
		{name: "custom local provider honors explicit workers", vision: localFakeVision{}, workers: 8, jobs: 48, want: 8},
		{name: "remote provider keeps explicit workers", vision: fakeVision{}, workers: 8, jobs: 48, want: 8},
		{name: "nil provider uses generic worker count", vision: nil, workers: 8, jobs: 48, want: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveOCRWorkersForVisionProvider(tt.workers, tt.jobs, tt.vision)
			if got != tt.want {
				t.Fatalf("EffectiveOCRWorkersForVisionProvider(%d, %d, %T) = %d, want %d", tt.workers, tt.jobs, tt.vision, got, tt.want)
			}
		})
	}
}
