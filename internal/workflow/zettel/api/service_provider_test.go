package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type fakeZettelProvider struct {
	mu              sync.Mutex
	id              string
	embeddingCalls  int
	embeddingActive int
	embeddingMax    int
	embeddingDelay  time.Duration
	embeddingErr    error
	chatCalls       []provider.ChatRequest
	chatResponse    string
	chatResponses   []string
	onChat          func(provider.ChatRequest)
}

func (f *fakeZettelProvider) ID() string {
	if f.id != "" {
		return f.id
	}
	return "fake"
}

func (f *fakeZettelProvider) Health(context.Context) error { return nil }

func (f *fakeZettelProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (f *fakeZettelProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chatCalls = append(f.chatCalls, req)
	if f.onChat != nil {
		f.onChat(req)
	}
	if len(f.chatResponses) > 0 {
		next := f.chatResponses[0]
		f.chatResponses = f.chatResponses[1:]
		return provider.ChatResponse{Content: next}, nil
	}
	if f.chatResponse == "" {
		return provider.ChatResponse{}, errors.New("chat should not be called")
	}
	return provider.ChatResponse{Content: f.chatResponse}, nil
}

func (f *fakeZettelProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (f *fakeZettelProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func (f *fakeZettelProvider) Embeddings(_ context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	f.mu.Lock()
	f.embeddingCalls++
	f.embeddingActive++
	if f.embeddingActive > f.embeddingMax {
		f.embeddingMax = f.embeddingActive
	}
	delay := f.embeddingDelay
	embeddingErr := f.embeddingErr
	f.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	defer func() {
		f.mu.Lock()
		f.embeddingActive--
		f.mu.Unlock()
	}()
	if embeddingErr != nil {
		return provider.EmbeddingResponse{}, embeddingErr
	}
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
	if resp.APICalls.Total != 1 || resp.APICalls.Embeddings != 1 || resp.APICalls.Chat != 0 {
		t.Fatalf("api calls = %#v, want one embedding call", resp.APICalls)
	}
}

func TestServiceIndexRunsEmbeddingBatchesConcurrently(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	for i := 0; i < 8; i++ {
		writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", fmt.Sprintf("note-%02d.md", i)), "# Active\n")
	}

	embeddingProvider := &fakeZettelProvider{embeddingDelay: 20 * time.Millisecond}
	resp, err := NewWithEmbedding(nil, embeddingProvider).Index(context.Background(), IndexRequest{
		Options: Options{
			VaultPath:          vaultDir,
			RootFolder:         "zettelkasten",
			DataFolder:         ".aicli-zettel-merge",
			EmbeddingModel:     "text-embedding-nomic-embed-text-v1.5",
			EmbeddingBatchSize: 1,
			EmbeddingWorkers:   4,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if resp.Updated != 8 {
		t.Fatalf("Index() = %#v, want eight updated notes", resp)
	}
	if embeddingProvider.embeddingMax < 2 {
		t.Fatalf("max concurrent embedding calls = %d, want concurrent batches", embeddingProvider.embeddingMax)
	}
}

func TestServiceIndexPrunesDeletedDestinationNotes(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	removedPath := filepath.Join(vaultDir, "zettelkasten", "removed.md")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "kept.md"), "# Kept\n")
	writeTestFile(t, removedPath, "# Removed\n")

	options := Options{
		VaultPath:      vaultDir,
		RootFolder:     "zettelkasten",
		DataFolder:     ".aicli-zettel-merge",
		EmbeddingModel: "text-embedding-nomic-embed-text-v1.5",
	}
	embeddingProvider := &fakeZettelProvider{}
	service := NewWithEmbedding(nil, embeddingProvider)
	first, err := service.Index(context.Background(), IndexRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("Index() first error = %v", err)
	}
	if first.Scanned != 2 || first.Updated != 2 || first.Pruned != 0 {
		t.Fatalf("first Index() = %#v, want two updated notes and no pruning", first)
	}

	if err := os.Remove(removedPath); err != nil {
		t.Fatalf("remove indexed note: %v", err)
	}
	second, err := service.Index(context.Background(), IndexRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("Index() second error = %v", err)
	}
	if second.Scanned != 1 || second.Reused != 1 || second.Updated != 0 || second.Pruned != 1 {
		t.Fatalf("second Index() = %#v, want one reused note and one pruned note", second)
	}

	v, err := vaultfs.New(vaultDir)
	if err != nil {
		t.Fatalf("newVault() error = %v", err)
	}
	cachedPaths, err := indexer.New(v, normalizeOptions(options), embeddingProvider).CachedPaths()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if _, ok := cachedPaths["zettelkasten/removed.md"]; ok {
		t.Fatalf("deleted note remains in cache: %#v", cachedPaths)
	}
	if _, ok := cachedPaths["zettelkasten/kept.md"]; !ok {
		t.Fatalf("kept note missing from cache: %#v", cachedPaths)
	}
}
