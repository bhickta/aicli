package zettel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

type embeddingCache struct {
	Version   int                      `json:"version"`
	Model     string                   `json:"model"`
	Items     map[string]embeddingItem `json:"items"`
	FullIndex map[string]any           `json:"full_index,omitempty"`
}

type embeddingItem struct {
	Hash      string    `json:"hash"`
	Embedding []float64 `json:"embedding"`
	UpdatedAt string    `json:"updated_at"`
}

type scoredCandidate struct {
	Path       string
	Content    string
	Similarity float64
}

type embeddingIndex struct {
	vault    vault
	options  Options
	provider provider.Provider
}

func newEmbeddingIndex(v vault, options Options, p provider.Provider) *embeddingIndex {
	return &embeddingIndex{vault: v, options: options, provider: p}
}

func (idx *embeddingIndex) Build(ctx context.Context, progress ProgressFunc) (IndexResponse, error) {
	notes, err := idx.vault.scanNotes(idx.options)
	if err != nil {
		return IndexResponse{}, err
	}
	cache, err := idx.load()
	if err != nil {
		return IndexResponse{}, err
	}
	resp := IndexResponse{Scanned: len(notes)}
	batch := make([]string, 0, idx.options.EmbeddingBatchSize)
	batchPaths := make([]string, 0, idx.options.EmbeddingBatchSize)
	batchHashes := make([]string, 0, idx.options.EmbeddingBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		vectors, err := idx.embed(ctx, batch)
		if err != nil {
			return err
		}
		for i, path := range batchPaths {
			cache.Items[path] = embeddingItem{
				Hash:      batchHashes[i],
				Embedding: vectors[i],
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			resp.Updated++
		}
		batch = batch[:0]
		batchPaths = batchPaths[:0]
		batchHashes = batchHashes[:0]
		return idx.save(cache)
	}
	for i, rel := range notes {
		if progress != nil && i%250 == 0 {
			progress(fmt.Sprintf("indexing zettelkasten embeddings (%d/%d)", i, len(notes)), 2, 4)
		}
		abs, err := idx.vault.notePath(rel, idx.options)
		if err != nil {
			return IndexResponse{}, err
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return IndexResponse{}, fmt.Errorf("read note %s: %w", rel, err)
		}
		source := compactNote(rel, string(content), idx.options.EmbeddingSourceChars)
		hash := hashText(idx.options.EmbeddingModel + "\n" + source)
		if item, ok := cache.Items[rel]; ok && item.Hash == hash && len(item.Embedding) > 0 {
			resp.Reused++
			continue
		}
		batch = append(batch, source)
		batchPaths = append(batchPaths, rel)
		batchHashes = append(batchHashes, hash)
		if len(batch) >= idx.options.EmbeddingBatchSize {
			if err := flush(); err != nil {
				return IndexResponse{}, err
			}
		}
	}
	if err := flush(); err != nil {
		return IndexResponse{}, err
	}
	cache.FullIndex = map[string]any{
		"completed":   true,
		"built_at":    time.Now().UTC().Format(time.RFC3339),
		"root_folder": idx.options.RootFolder,
		"file_count":  len(notes),
	}
	if err := idx.save(cache); err != nil {
		return IndexResponse{}, err
	}
	return resp, nil
}

func (idx *embeddingIndex) Similar(ctx context.Context, activePath string, activeContent string) ([]scoredCandidate, error) {
	cache, err := idx.load()
	if err != nil {
		return nil, err
	}
	if len(cache.Items) == 0 {
		return nil, errors.New("embedding index is empty; run the zettel index workflow first")
	}
	activeSource := compactNote(activePath, activeContent, idx.options.EmbeddingSourceChars)
	activeVectors, err := idx.embed(ctx, []string{activeSource})
	if err != nil {
		return nil, err
	}
	activeVector := activeVectors[0]
	scored := make([]scoredCandidate, 0, len(cache.Items))
	for path, item := range cache.Items {
		if path == activePath || !isInScope(path, idx.options) || len(item.Embedding) == 0 {
			continue
		}
		abs, err := idx.vault.notePath(path, idx.options)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		scored = append(scored, scoredCandidate{
			Path:       path,
			Content:    string(content),
			Similarity: cosineSimilarity(activeVector, item.Embedding),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})
	if len(scored) > idx.options.CandidateLimit {
		scored = scored[:idx.options.CandidateLimit]
	}
	return scored, nil
}

func (idx *embeddingIndex) load() (embeddingCache, error) {
	cache := embeddingCache{Version: 1, Model: idx.options.EmbeddingModel, Items: map[string]embeddingItem{}}
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
		return embeddingCache{Version: 1, Model: idx.options.EmbeddingModel, Items: map[string]embeddingItem{}}, nil
	}
	if cache.Items == nil {
		cache.Items = map[string]embeddingItem{}
	}
	return cache, nil
}

func (idx *embeddingIndex) save(cache embeddingCache) error {
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

func (idx *embeddingIndex) cachePath(legacy bool) (string, error) {
	dataFolder := idx.options.DataFolder
	if legacy {
		dataFolder = ".zettel-merge-ai"
	}
	return idx.vault.abs(filepath.Join(dataFolder, "index", "embeddings.json"))
}

func (idx *embeddingIndex) embed(ctx context.Context, inputs []string) ([][]float64, error) {
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

func cosineSimilarity(a []float64, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
