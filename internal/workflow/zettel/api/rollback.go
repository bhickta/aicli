package zettel

import (
	"context"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (s *Service) Rollback(_ context.Context, req RollbackRequest, progress ProgressFunc) (RollbackResponse, error) {
	req.Options = s.workflowOptions(req.Options)
	v, err := vaultfs.New(req.Options.VaultPath)
	if err != nil {
		return RollbackResponse{}, err
	}
	if progress != nil {
		progress(progressmodel.Indeterminate("restoring latest zettel inbox merge archive"))
	}
	jobID, err := archivepkg.NewStore(v, req.Options).Rollback(req.JobID)
	if err != nil {
		return RollbackResponse{}, err
	}
	return RollbackResponse{JobID: jobID}, nil
}
