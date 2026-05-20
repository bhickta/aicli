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
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/systemresources"
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

type preparedEmbeddingSource struct {
	index  int
	path   string
	source string
	hash   string
	reused bool
	err    error
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
	prepared, reused, err := idx.prepareEmbeddingSources(ctx, notes, cache, progress)
	if err != nil {
		return IndexResponse{}, err
	}
	resp.Reused = reused
	batch := make([]string, 0, idx.options.EmbeddingBatchSize)
	batchPaths := make([]string, 0, idx.options.EmbeddingBatchSize)
	batchHashes := make([]string, 0, idx.options.EmbeddingBatchSize)
	flushes := 0
	flush := func(forceSave bool) error {
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
		flushes++
		if forceSave || flushes%4 == 0 {
			return idx.save(cache)
		}
		return nil
	}
	for i, item := range prepared {
		if progress != nil && i%250 == 0 {
			progress(fmt.Sprintf("indexing zettelkasten embeddings (%d/%d)", i, len(notes)), 2, 4)
		}
		if item.reused {
			continue
		}
		batch = append(batch, item.source)
		batchPaths = append(batchPaths, item.path)
		batchHashes = append(batchHashes, item.hash)
		if len(batch) >= idx.options.EmbeddingBatchSize {
			if err := flush(false); err != nil {
				return IndexResponse{}, err
			}
		}
	}
	if err := flush(true); err != nil {
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

func (idx *embeddingIndex) prepareEmbeddingSources(ctx context.Context, notes []string, cache embeddingCache, progress ProgressFunc) ([]preparedEmbeddingSource, int, error) {
	workers := systemresources.DefaultZettelReadWorkers(systemresources.Snapshot{})
	if workers > len(notes) {
		workers = len(notes)
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan int)
	results := make(chan preparedEmbeddingSource, len(notes))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					results <- preparedEmbeddingSource{index: index, err: ctx.Err()}
					continue
				default:
				}
				results <- idx.prepareEmbeddingSource(notes[index], index, cache)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range notes {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	prepared := make([]preparedEmbeddingSource, len(notes))
	reused := 0
	seen := 0
	for result := range results {
		if result.err != nil {
			return nil, 0, result.err
		}
		prepared[result.index] = result
		if result.reused {
			reused++
		}
		seen++
		if progress != nil && seen%500 == 0 {
			progress(fmt.Sprintf("reading zettelkasten notes (%d/%d)", seen, len(notes)), 1, 4)
		}
	}
	if seen != len(notes) {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		return nil, 0, errors.New("zettel index stopped before all notes were read")
	}
	return prepared, reused, nil
}

func (idx *embeddingIndex) prepareEmbeddingSource(rel string, index int, cache embeddingCache) preparedEmbeddingSource {
	abs, err := idx.vault.notePath(rel, idx.options)
	if err != nil {
		return preparedEmbeddingSource{index: index, err: err}
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return preparedEmbeddingSource{index: index, err: fmt.Errorf("read note %s: %w", rel, err)}
	}
	source := compactNote(rel, string(content), idx.options.EmbeddingSourceChars)
	hash := hashText(idx.options.EmbeddingModel + "\n" + source)
	if item, ok := cache.Items[rel]; ok && item.Hash == hash && len(item.Embedding) > 0 {
		return preparedEmbeddingSource{index: index, path: rel, reused: true}
	}
	return preparedEmbeddingSource{index: index, path: rel, source: source, hash: hash}
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
	scored := idx.scoreCandidates(ctx, activePath, activeVector, cache)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})
	if len(scored) > idx.options.CandidateLimit {
		scored = scored[:idx.options.CandidateLimit]
	}
	return scored, nil
}

func (idx *embeddingIndex) scoreCandidates(ctx context.Context, activePath string, activeVector []float64, cache embeddingCache) []scoredCandidate {
	type candidateItem struct {
		path string
		item embeddingItem
	}
	candidates := make([]candidateItem, 0, len(cache.Items))
	for path, item := range cache.Items {
		if path == activePath || !isInScope(path, idx.options) || len(item.Embedding) == 0 {
			continue
		}
		candidates = append(candidates, candidateItem{path: path, item: item})
	}
	workers := systemresources.DefaultZettelReadWorkers(systemresources.Snapshot{})
	if workers > len(candidates) {
		workers = len(candidates)
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan candidateItem)
	results := make(chan scoredCandidate, len(candidates))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				abs, err := idx.vault.notePath(candidate.path, idx.options)
				if err != nil {
					continue
				}
				content, err := os.ReadFile(abs)
				if err != nil {
					continue
				}
				results <- scoredCandidate{
					Path:       candidate.path,
					Content:    string(content),
					Similarity: cosineSimilarity(activeVector, candidate.item.Embedding),
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, candidate := range candidates {
			select {
			case <-ctx.Done():
				return
			case jobs <- candidate:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	scored := make([]scoredCandidate, 0, len(candidates))
	for item := range results {
		scored = append(scored, item)
	}
	return scored
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
