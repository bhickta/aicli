package zettel

import (
	"context"

	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	metadatapkg "github.com/bhickta/aicli/internal/workflow/zettel/metadata"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (s *Service) Metadata(ctx context.Context, req MetadataRequest, progress ProgressFunc) (MetadataResponse, error) {
	req.Options = s.workflowOptions(req.Options)
	tracker, mergeProvider, _ := s.trackedProviders()
	response, err := metadatapkg.New(mergeProvider).Generate(ctx, req, progress)
	response.APICalls = tracker.Snapshot()
	if err == nil && response.RunID != "" {
		options := s.workflowOptions(req.Options)
		v, vaultErr := vaultfs.New(options.VaultPath)
		if vaultErr != nil {
			return response, vaultErr
		}
		if updateErr := archivepkg.NewStore(v, options).UpdateMetadataRunAPICalls(response.RunID, response.APICalls); updateErr != nil {
			return response, updateErr
		}
	}
	return response, err
}
