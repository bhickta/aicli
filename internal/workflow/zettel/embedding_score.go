package zettel

import (
	"context"
	"math"
	"os"
	"sync"

	"github.com/bhickta/aicli/internal/systemresources"
)

type embeddingCandidate struct {
	path string
	item embeddingItem
}

func (idx *embeddingIndex) scoreCandidates(
	ctx context.Context,
	activePath string,
	activeVector []float64,
	cache embeddingCache,
) []scoredCandidate {
	candidates := idx.embeddingCandidates(activePath, cache)
	workers := normalizedEmbeddingScoreWorkers(len(candidates))
	jobs := make(chan embeddingCandidate)
	results := make(chan scoredCandidate, len(candidates))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.scoreCandidateWorker(ctx, activeVector, jobs, results)
		}()
	}

	go sendEmbeddingCandidates(ctx, candidates, jobs)
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

func (idx *embeddingIndex) embeddingCandidates(activePath string, cache embeddingCache) []embeddingCandidate {
	candidates := make([]embeddingCandidate, 0, len(cache.Items))
	for path, item := range cache.Items {
		if path == activePath || !isInScope(path, idx.options) || len(item.Embedding) == 0 {
			continue
		}
		candidates = append(candidates, embeddingCandidate{path: path, item: item})
	}
	return candidates
}

func normalizedEmbeddingScoreWorkers(candidateCount int) int {
	workers := systemresources.DefaultZettelReadWorkers(systemresources.Snapshot{})
	if workers > candidateCount {
		workers = candidateCount
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func sendEmbeddingCandidates(
	ctx context.Context,
	candidates []embeddingCandidate,
	jobs chan<- embeddingCandidate,
) {
	defer close(jobs)
	for _, candidate := range candidates {
		select {
		case <-ctx.Done():
			return
		case jobs <- candidate:
		}
	}
}

func (idx *embeddingIndex) scoreCandidateWorker(
	ctx context.Context,
	activeVector []float64,
	jobs <-chan embeddingCandidate,
	results chan<- scoredCandidate,
) {
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
