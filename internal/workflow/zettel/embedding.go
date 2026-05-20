package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
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
	resp.Pruned = pruneMissingCacheItems(&cache, notes)
	prepared, reused, err := idx.prepareEmbeddingSources(ctx, notes, cache, progress)
	if err != nil {
		return IndexResponse{}, err
	}
	resp.Reused = reused
	pending := 0
	for _, item := range prepared {
		if !item.reused {
			pending++
		}
	}
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
			progress(progressmodel.Units(
				fmt.Sprintf("indexing zettelkasten embeddings (%d/%d)", resp.Updated, pending),
				resp.Updated,
				pending,
				"embedding",
			))
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

func pruneMissingCacheItems(cache *embeddingCache, notes []string) int {
	if len(cache.Items) == 0 {
		return 0
	}
	current := make(map[string]struct{}, len(notes))
	for _, note := range notes {
		current[note] = struct{}{}
	}
	pruned := 0
	for path := range cache.Items {
		if _, ok := current[path]; ok {
			continue
		}
		delete(cache.Items, path)
		pruned++
	}
	return pruned
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
			progress(progressmodel.Units(
				fmt.Sprintf("reading zettelkasten notes (%d/%d)", seen, len(notes)),
				seen,
				len(notes),
				"note",
			))
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
