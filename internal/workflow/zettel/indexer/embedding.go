package indexer

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
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Options = model.Options
type IndexResponse = model.IndexResponse
type ProgressFunc = model.ProgressFunc

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

type ScoredCandidate struct {
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

type embeddingBatch struct {
	inputs []string
	paths  []string
	hashes []string
}

type embeddingBatchResult struct {
	batch   embeddingBatch
	vectors [][]float64
	err     error
}

type Index struct {
	vault    vaultfs.Vault
	options  Options
	provider provider.Provider
}

func New(v vaultfs.Vault, options Options, p provider.Provider) *Index {
	return &Index{vault: v, options: options, provider: p}
}

func (idx *Index) Build(ctx context.Context, progress ProgressFunc) (IndexResponse, error) {
	notes, err := idx.vault.ScanNotes(idx.options)
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
	batches := buildEmbeddingBatches(prepared, idx.options.EmbeddingBatchSize)
	if err := idx.embedBatches(ctx, batches, &cache, &resp, pending, progress); err != nil {
		return IndexResponse{}, err
	}
	cache.FullIndex = map[string]any{
		"completed":         true,
		"built_at":          time.Now().UTC().Format(time.RFC3339),
		"root_folder":       idx.options.RootFolder,
		"file_count":        len(notes),
		"embedding_batch":   idx.options.EmbeddingBatchSize,
		"embedding_workers": idx.options.EmbeddingWorkers,
	}
	if err := idx.save(cache); err != nil {
		return IndexResponse{}, err
	}
	return resp, nil
}

func buildEmbeddingBatches(prepared []preparedEmbeddingSource, batchSize int) []embeddingBatch {
	if batchSize < 1 {
		batchSize = model.DefaultEmbeddingBatchSize
	}
	batches := []embeddingBatch{}
	current := embeddingBatch{
		inputs: make([]string, 0, batchSize),
		paths:  make([]string, 0, batchSize),
		hashes: make([]string, 0, batchSize),
	}
	for _, item := range prepared {
		if item.reused {
			continue
		}
		current.inputs = append(current.inputs, item.source)
		current.paths = append(current.paths, item.path)
		current.hashes = append(current.hashes, item.hash)
		if len(current.inputs) >= batchSize {
			batches = append(batches, current)
			current = embeddingBatch{
				inputs: make([]string, 0, batchSize),
				paths:  make([]string, 0, batchSize),
				hashes: make([]string, 0, batchSize),
			}
		}
	}
	if len(current.inputs) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func (idx *Index) embedBatches(
	ctx context.Context,
	batches []embeddingBatch,
	cache *embeddingCache,
	resp *IndexResponse,
	pending int,
	progress ProgressFunc,
) error {
	if len(batches) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workers := idx.embeddingWorkers(len(batches))
	jobs := make(chan embeddingBatch)
	results := make(chan embeddingBatchResult, len(batches))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				vectors, err := idx.embed(ctx, batch.inputs)
				results <- embeddingBatchResult{
					batch:   batch,
					vectors: vectors,
					err:     err,
				}
				if err != nil {
					cancel()
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, batch := range batches {
			select {
			case <-ctx.Done():
				return
			case jobs <- batch:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	flushes := 0
	for result := range results {
		if result.err != nil {
			return result.err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		for i, path := range result.batch.paths {
			cache.Items[path] = embeddingItem{
				Hash:      result.batch.hashes[i],
				Embedding: result.vectors[i],
				UpdatedAt: now,
			}
			resp.Updated++
		}
		flushes++
		if progress != nil {
			progress(progressmodel.Units(
				fmt.Sprintf(
					"indexing zettelkasten embeddings (%d/%d, %d workers)",
					resp.Updated,
					pending,
					workers,
				),
				resp.Updated,
				pending,
				"embedding",
			))
		}
		if flushes%4 == 0 {
			if err := idx.save(*cache); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (idx *Index) embeddingWorkers(batchCount int) int {
	workers := idx.options.EmbeddingWorkers
	if workers < 1 {
		workers = model.DefaultEmbeddingWorkers
	}
	if workers > batchCount {
		workers = batchCount
	}
	if workers < 1 {
		return 1
	}
	return workers
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

func (idx *Index) prepareEmbeddingSources(ctx context.Context, notes []string, cache embeddingCache, progress ProgressFunc) ([]preparedEmbeddingSource, int, error) {
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

func (idx *Index) prepareEmbeddingSource(rel string, index int, cache embeddingCache) preparedEmbeddingSource {
	abs, err := idx.vault.NotePath(rel, idx.options)
	if err != nil {
		return preparedEmbeddingSource{index: index, err: err}
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return preparedEmbeddingSource{index: index, err: fmt.Errorf("read note %s: %w", rel, err)}
	}
	source := notetext.CompactNote(rel, string(content), idx.options.EmbeddingSourceChars)
	hash := notetext.HashText(idx.options.EmbeddingModel + "\n" + source)
	if item, ok := cache.Items[rel]; ok && item.Hash == hash && len(item.Embedding) > 0 {
		return preparedEmbeddingSource{index: index, path: rel, reused: true}
	}
	return preparedEmbeddingSource{index: index, path: rel, source: source, hash: hash}
}

func (idx *Index) Similar(ctx context.Context, activePath string, activeContent string) ([]ScoredCandidate, error) {
	cache, err := idx.load()
	if err != nil {
		return nil, err
	}
	if len(cache.Items) == 0 {
		return nil, errors.New("embedding index is empty; run the zettel index workflow first")
	}
	activeSource := notetext.CompactNote(activePath, activeContent, idx.options.EmbeddingSourceChars)
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
