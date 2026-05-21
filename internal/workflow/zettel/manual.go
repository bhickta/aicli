package zettel

import (
	"context"

	manualpkg "github.com/bhickta/aicli/internal/workflow/zettel/manual"
)

func (s *Service) Suggest(ctx context.Context, req SuggestRequest, progress ProgressFunc) (SuggestResponse, error) {
	return s.manualRunner().Suggest(ctx, req, progress)
}

func (s *Service) Propose(ctx context.Context, req ProposeRequest, progress ProgressFunc) (ProposeResponse, error) {
	return s.manualRunner().Propose(ctx, req, progress)
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
