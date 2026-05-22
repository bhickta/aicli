package zettelapi

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	zettel "github.com/bhickta/aicli/internal/workflow/zettel/api"
)

type zettelAPITestProvider struct {
	id        string
	embedding bool
}

func (p zettelAPITestProvider) ID() string {
	return p.id
}

func (p zettelAPITestProvider) Health(context.Context) error {
	return nil
}

func (p zettelAPITestProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (p zettelAPITestProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("chat should not be called")
}

func (p zettelAPITestProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (p zettelAPITestProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func (p zettelAPITestProvider) Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	if !p.embedding {
		return provider.EmbeddingResponse{}, errors.New("embeddings should not be called")
	}
	return provider.EmbeddingResponse{Vectors: [][]float64{{1, 1}}}, nil
}

func TestServiceForIndexUsesResolvedEmbeddingProviderBeforeFallback(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeZettelAPITestFile(t, filepath.Join(vaultDir, "zettelkasten", "note.md"), "# Note\n")
	providers := map[string]provider.Provider{
		"embed": zettelAPITestProvider{id: "embed", embedding: true},
		"chat":  noEmbeddingZettelAPITestProvider{id: "chat"},
	}
	handler := New(core.New(core.Dependencies{
		Logger:  slog.Default(),
		DataDir: t.TempDir(),
		Settings: func() config.Settings {
			return config.Settings{DefaultProvider: "chat"}
		},
		ProviderFor: func(id string) (provider.Provider, bool) {
			p, ok := providers[id]
			return p, ok
		},
	}))
	service, err := handler.serviceFor(zettel.Options{
		VaultPath:  vaultDir,
		ProviderID: "embed",
	}, providerNeeds{embedding: true})
	if err != nil {
		t.Fatalf("serviceFor() error = %v", err)
	}

	resp, err := service.Index(context.Background(), zettel.IndexRequest{
		Options: zettel.Options{
			VaultPath:  vaultDir,
			ProviderID: "embed",
		},
	}, nil)
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("Index() = %#v, want one embedded note", resp)
	}
}

type noEmbeddingZettelAPITestProvider struct {
	id string
}

func (p noEmbeddingZettelAPITestProvider) ID() string {
	return p.id
}

func (p noEmbeddingZettelAPITestProvider) Health(context.Context) error {
	return nil
}

func (p noEmbeddingZettelAPITestProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (p noEmbeddingZettelAPITestProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("chat should not be called")
}

func (p noEmbeddingZettelAPITestProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (p noEmbeddingZettelAPITestProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func writeZettelAPITestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
