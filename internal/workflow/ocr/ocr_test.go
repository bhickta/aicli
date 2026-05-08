package ocr

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeVisionProvider struct {
	count int
}

func (f *fakeVisionProvider) ID() string { return "fake" }
func (f *fakeVisionProvider) Health(context.Context) error {
	return nil
}
func (f *fakeVisionProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (f *fakeVisionProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
func (f *fakeVisionProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (f *fakeVisionProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	f.count++
	return provider.ChatResponse{Content: "page text"}, nil
}

func TestRunProcessesImageZip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pages.zip")
	createZip(t, path, map[string]string{"page2.jpg": "b", "page1.png": "a", "ignore.txt": "x"})

	fp := &fakeVisionProvider{}
	res, err := New(fp).Run(context.Background(), Request{Path: path, Model: "vision"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if fp.count != 2 {
		t.Fatalf("vision calls = %d, want 2", fp.count)
	}
	if len(res.Pages) != 2 || res.Markdown == "" {
		t.Fatalf("Response = %#v, want pages and markdown", res)
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	target, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	writer := zip.NewWriter(target)
	defer writer.Close()
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}
