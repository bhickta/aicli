package zettel

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeZettelProvider struct {
	embeddingCalls int
}

func (f *fakeZettelProvider) ID() string { return "fake" }

func (f *fakeZettelProvider) Health(context.Context) error { return nil }

func (f *fakeZettelProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (f *fakeZettelProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("chat should not be called")
}

func (f *fakeZettelProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (f *fakeZettelProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func (f *fakeZettelProvider) Embeddings(_ context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	f.embeddingCalls++
	vectors := make([][]float64, len(req.Inputs))
	for i := range req.Inputs {
		vectors[i] = []float64{1, float64(i + 1)}
	}
	return provider.EmbeddingResponse{Vectors: vectors}, nil
}

func TestServiceIndexUsesSeparateEmbeddingProvider(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"), "# Active\n")

	embeddingProvider := &fakeZettelProvider{}
	resp, err := NewWithEmbedding(nil, embeddingProvider).Index(context.Background(), IndexRequest{
		Options: Options{
			VaultPath:      vaultDir,
			RootFolder:     "zettelkasten",
			DataFolder:     ".aicli-zettel-merge",
			ProviderID:     "codex-cli",
			JudgeModel:     "gpt-5.3-codex-spark",
			MergeModel:     "gpt-5.3-codex-spark",
			EmbeddingModel: "text-embedding-nomic-embed-text-v1.5",
		},
	}, nil)
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if resp.Updated != 1 || embeddingProvider.embeddingCalls != 1 {
		t.Fatalf("Index() = %#v, embedding calls = %d; want one updated note through embedding provider", resp, embeddingProvider.embeddingCalls)
	}
}
