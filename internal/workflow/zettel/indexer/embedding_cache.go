package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bhickta/aicli/internal/provider"
)

type embedder interface {
	Embeddings(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
}

func (idx *Index) load() (embeddingCache, error) {
	cache := embeddingCache{
		Version: 1,
		Model:   idx.options.EmbeddingModel,
		Items:   map[string]embeddingItem{},
	}

	path, err := idx.cachePath(false)
	if err != nil {
		return cache, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy, legacyErr := idx.cachePath(true)
		if legacyErr != nil {
			return cache, nil
		}
		data, err = os.ReadFile(legacy)
	}
	if errors.Is(err, os.ErrNotExist) {
		return cache, nil
	}
	if err != nil {
		return cache, fmt.Errorf("read embedding cache: %w", err)
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return embeddingCache{}, fmt.Errorf("parse embedding cache: %w", err)
	}
	if cache.Model != idx.options.EmbeddingModel {
		return embeddingCache{
			Version: 1,
			Model:   idx.options.EmbeddingModel,
			Items:   map[string]embeddingItem{},
		}, nil
	}
	if cache.Items == nil {
		cache.Items = map[string]embeddingItem{}
	}
	return cache, nil
}

func (idx *Index) CachedItemCount() (int, error) {
	cache, err := idx.load()
	if err != nil {
		return 0, err
	}
	return len(cache.Items), nil
}

func (idx *Index) CachedPaths() (map[string]struct{}, error) {
	cache, err := idx.load()
	if err != nil {
		return nil, err
	}
	paths := make(map[string]struct{}, len(cache.Items))
	for path := range cache.Items {
		paths[path] = struct{}{}
	}
	return paths, nil
}

func (idx *Index) CheckEmbeddingProvider(ctx context.Context) error {
	_, err := idx.embed(ctx, []string{"zettelkasten inbox merge embedding preflight"})
	return err
}

func (idx *Index) save(cache embeddingCache) error {
	path, err := idx.cachePath(false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create embedding cache folder: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (idx *Index) cachePath(legacy bool) (string, error) {
	dataFolder := idx.options.DataFolder
	if legacy {
		dataFolder = ".zettel-merge-ai"
	}
	return idx.vault.Abs(filepath.Join(dataFolder, "index", "embeddings.json"))
}

func (idx *Index) embed(ctx context.Context, inputs []string) ([][]float64, error) {
	embedder, ok := idx.provider.(embedder)
	if !ok {
		return nil, errors.New("selected embedding provider does not support embeddings")
	}

	resp, err := embedder.Embeddings(ctx, provider.EmbeddingRequest{
		Model:  idx.options.EmbeddingModel,
		Inputs: inputs,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Vectors) != len(inputs) {
		return nil, fmt.Errorf("embedding provider returned %d vector(s) for %d input(s)", len(resp.Vectors), len(inputs))
	}
	return resp.Vectors, nil
}
