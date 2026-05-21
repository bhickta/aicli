package zettel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeZettelProvider struct {
	id             string
	embeddingCalls int
	embeddingErr   error
	chatCalls      []provider.ChatRequest
	chatResponse   string
	chatResponses  []string
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
	f.chatCalls = append(f.chatCalls, req)
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
	f.embeddingCalls++
	if f.embeddingErr != nil {
		return provider.EmbeddingResponse{}, f.embeddingErr
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

	v, err := newVault(vaultDir)
	if err != nil {
		t.Fatalf("newVault() error = %v", err)
	}
	cachedPaths, err := newEmbeddingIndex(v, normalizeOptions(options), embeddingProvider).CachedPaths()
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

func TestServiceProposeUsesSeparateStepProvidersAndModels(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"), "# Active\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "source.md"), "- copied fact\n")

	candidateProvider := &fakeZettelProvider{id: "candidate"}
	mergeProvider := &fakeZettelProvider{
		id:           "merge",
		chatResponse: `{"insertions":[{"after_line":1,"markdown":"- copied fact","reason":"keep fact"}],"notes":"ok"}`,
	}
	validationProvider := &fakeZettelProvider{
		id:           "validation",
		chatResponse: `{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"ok"}`,
	}
	embeddingProvider := &fakeZettelProvider{id: "embedding"}

	resp, err := NewWithProviders(
		candidateProvider,
		mergeProvider,
		validationProvider,
		embeddingProvider,
	).Propose(context.Background(), ProposeRequest{
		Options: Options{
			VaultPath:            vaultDir,
			RootFolder:           "zettelkasten",
			DataFolder:           ".aicli-zettel-merge",
			CandidateProviderID:  "candidate",
			MergeProviderID:      "merge",
			ValidationProviderID: "validation",
			EmbeddingProviderID:  "embedding",
			CandidateModel:       "candidate-model",
			MergeModel:           "merge-model",
			ValidationModel:      "validation-model",
			EmbeddingModel:       "embedding-model",
		},
		ActivePath: "zettelkasten/active.md",
		Selections: []Selection{{
			Path:             "zettelkasten/source.md",
			SourceLineRanges: []LineRange{{StartLine: 1, EndLine: 1}},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if len(candidateProvider.chatCalls) != 0 {
		t.Fatalf("candidate provider chat calls = %d, want 0 during propose", len(candidateProvider.chatCalls))
	}
	if len(mergeProvider.chatCalls) != 1 || mergeProvider.chatCalls[0].Model != "merge-model" {
		t.Fatalf("merge calls = %#v, want one merge-model call", mergeProvider.chatCalls)
	}
	if len(validationProvider.chatCalls) != 1 || validationProvider.chatCalls[0].Model != "validation-model" {
		t.Fatalf("validation calls = %#v, want one validation-model call", validationProvider.chatCalls)
	}

	proposal := resp.Proposal
	if proposal.Models.Merge != "merge-model" || proposal.Models.ValidationJudge != "validation-model" {
		t.Fatalf("proposal models = %#v, want step models recorded", proposal.Models)
	}
	if proposal.Providers.Merge != "merge" || proposal.Providers.ValidationJudge != "validation" || proposal.Providers.Embedding != "embedding" {
		t.Fatalf("proposal providers = %#v, want step providers recorded", proposal.Providers)
	}
}
