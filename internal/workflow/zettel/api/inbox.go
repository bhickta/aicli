package zettel

import (
	"context"

	inboxpkg "github.com/bhickta/aicli/internal/workflow/zettel/inbox"
)

func (s *Service) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	tracker, candidateProvider, mergeProvider, validationProvider, embeddingProvider := s.trackedProviders()
	response, err := inboxpkg.New(
		candidateProvider,
		mergeProvider,
		validationProvider,
		embeddingProvider,
	).InboxMerge(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	return response, err
}
