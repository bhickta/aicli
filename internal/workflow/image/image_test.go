package image

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeVisionProvider struct {
	req provider.VisionRequest
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
func (f *fakeVisionProvider) Vision(_ context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	f.req = req
	return provider.ChatResponse{Content: "useful-file-name"}, nil
}

func TestRunImageRename(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "image.jpg")
	if err := os.WriteFile(path, []byte("fake image"), 0o600); err != nil {
		t.Fatal(err)
	}

	fp := &fakeVisionProvider{}
	res, err := New(fp).Run(context.Background(), Request{Path: path, Mode: "rename", Model: "vision"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Text != "useful-file-name" {
		t.Fatalf("Text = %q, want useful-file-name", res.Text)
	}
	if fp.req.Model != "vision" || len(fp.req.Image) == 0 {
		t.Fatalf("VisionRequest = %#v, want model and image", fp.req)
	}
}

func TestRenameCanApplySafeFilename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "old.jpg")
	if err := os.WriteFile(path, []byte("fake image"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := New(&fakeVisionProvider{}).Rename(context.Background(), RenameRequest{Path: path, Apply: true})
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if !res.Applied {
		t.Fatal("Applied = false, want true")
	}
	if _, err := os.Stat(res.NewPath); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
}

func TestPruneRefsDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	md := filepath.Join(dir, "doc.md")
	assets := filepath.Join(dir, "assets")
	if err := os.Mkdir(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, []byte("![x](assets/keep.png)"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "keep.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "drop.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := PruneRefs(PruneRefsRequest{MarkdownPath: md, AssetDir: assets})
	if err != nil {
		t.Fatalf("PruneRefs() error = %v", err)
	}
	if len(res.Removed) != 1 {
		t.Fatalf("Removed = %#v, want one stale asset", res.Removed)
	}
}
