package zettel

import (
	"context"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Service struct {
	candidateProvider  provider.Provider
	mergeProvider      provider.Provider
	validationProvider provider.Provider
	embeddingProvider  provider.Provider
}

func New(p provider.Provider) *Service {
	return NewWithEmbedding(p, p)
}

func NewWithEmbedding(p provider.Provider, embeddingProvider provider.Provider) *Service {
	return NewWithProviders(p, p, p, embeddingProvider)
}

func NewWithProviders(
	candidateProvider provider.Provider,
	mergeProvider provider.Provider,
	validationProvider provider.Provider,
	embeddingProvider provider.Provider,
) *Service {
	return &Service{
		candidateProvider:  candidateProvider,
		mergeProvider:      mergeProvider,
		validationProvider: validationProvider,
		embeddingProvider:  embeddingProvider,
	}
}

func (s *Service) Index(ctx context.Context, req IndexRequest, progress ProgressFunc) (IndexResponse, error) {
	options := normalizeOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
	if err != nil {
		return IndexResponse{}, err
	}
	return indexer.New(v, options, s.embeddingProvider).Build(ctx, progress)
}
