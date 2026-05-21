package zettel

import (
	"context"

	inboxpkg "github.com/bhickta/aicli/internal/workflow/zettel/inbox"
)

func (s *Service) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	return inboxpkg.New(
		s.candidateProvider,
		s.mergeProvider,
		s.validationProvider,
		s.embeddingProvider,
	).InboxMerge(ctx, req, progress)
}
