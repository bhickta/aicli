package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct{}

func (fakeRunner) CombinedOutput(_ context.Context, _ string, args ...string) ([]byte, error) {
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

func TestRenderPDFToImages(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	images, cleanup, err := RenderPDFToImages(context.Background(), config.ToolConfig{PDFToPPM: "pdftoppm"}, fakeRunner{}, pdf, 200)
	if err != nil {
		t.Fatalf("RenderPDFToImages() error = %v", err)
	}
	defer cleanup()
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
	)
	if err != nil {
		t.Fatalf("OCRImages() error = %v", err)
	}
	if pages[0].Name != "b" || pages[1].Name != "a" {
		t.Fatalf("pages = %#v, want input order preserved", pages)
	}
}
