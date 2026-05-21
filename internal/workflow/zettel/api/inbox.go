package zettel

import (
	"context"

	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	inboxpkg "github.com/bhickta/aicli/internal/workflow/zettel/inbox"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (s *Service) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	tracker, _, mergeProvider, _, embeddingProvider := s.trackedProviders()
	response, err := inboxpkg.New(
		mergeProvider,
		embeddingProvider,
	).InboxMerge(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	if err == nil && response.RunID != "" {
		options := normalizeOptions(req.Options)
		v, vaultErr := vaultfs.New(options.VaultPath)
		if vaultErr != nil {
			return response, vaultErr
		}
		if updateErr := archivepkg.NewStore(v, options).UpdateInboxRunAPICalls(response.RunID, response.APICalls); updateErr != nil {
			return response, updateErr
		}
	}
	return response, err
}
