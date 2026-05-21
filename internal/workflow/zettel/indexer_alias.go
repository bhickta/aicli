package zettel

import (
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
)

type scoredCandidate = indexer.ScoredCandidate

func newEmbeddingIndex(v vault, options Options, p provider.Provider) *indexer.Index {
	return indexer.New(v, options, p)
}
