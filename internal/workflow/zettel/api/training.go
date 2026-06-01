package zettel

import (
	"context"

	trainingpkg "github.com/bhickta/aicli/internal/workflow/zettel/training"
)

func (s *Service) TrainingExport(
	ctx context.Context,
	req TrainingExportRequest,
	progress ProgressFunc,
) (TrainingExportResponse, error) {
	req.Options = s.workflowOptions(req.Options)
	return trainingpkg.New().Export(ctx, req, progress)
}
