package zettel

import (
	"context"

	inboxpkg "github.com/bhickta/aicli/internal/workflow/zettel/inbox"
)

func (s *Service) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	tracker, _, mergeProvider, _, embeddingProvider := s.trackedProviders()
	response, err := inboxpkg.New(
		mergeProvider,
		embeddingProvider,
	).InboxMerge(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	return response, err
}
