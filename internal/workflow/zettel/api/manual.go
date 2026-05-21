package zettel

import (
	"context"

	manualpkg "github.com/bhickta/aicli/internal/workflow/zettel/manual"
)

func (s *Service) Suggest(ctx context.Context, req SuggestRequest, progress ProgressFunc) (SuggestResponse, error) {
	tracker, candidateProvider, mergeProvider, validationProvider, embeddingProvider := s.trackedProviders()
	response, err := manualpkg.New(
		candidateProvider,
		mergeProvider,
		validationProvider,
		embeddingProvider,
	).Suggest(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	return response, err
}

func (s *Service) Propose(ctx context.Context, req ProposeRequest, progress ProgressFunc) (ProposeResponse, error) {
	tracker, candidateProvider, mergeProvider, validationProvider, embeddingProvider := s.trackedProviders()
	response, err := manualpkg.New(
		candidateProvider,
		mergeProvider,
		validationProvider,
		embeddingProvider,
	).Propose(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	response.Proposal.APICalls = response.APICalls
	return response, err
}

func (s *Service) Apply(ctx context.Context, req ApplyRequest, progress ProgressFunc) (ApplyResponse, error) {
	return s.manualRunner().Apply(ctx, req, progress)
}

func (s *Service) Rollback(ctx context.Context, req RollbackRequest, progress ProgressFunc) (RollbackResponse, error) {
	return s.manualRunner().Rollback(ctx, req, progress)
}

func (s *Service) manualRunner() manualpkg.Runner {
	return manualpkg.New(
		s.candidateProvider,
		s.mergeProvider,
		s.validationProvider,
		s.embeddingProvider,
	)
}
