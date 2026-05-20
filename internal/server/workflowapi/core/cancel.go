package core

import (
	"context"
	"errors"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

func (r *Runtime) CancelJob(ctx context.Context, jobID string) (storage.Job, error) {
	if jobID == "" {
		return storage.Job{}, errors.New("job id is required")
	}
	job, err := r.store.GetJob(ctx, jobID)
	if err != nil {
		return storage.Job{}, err
	}
	if job.Status != "running" {
		return job, nil
	}
	if cancel, ok := r.cancelFunc(jobID); ok {
		cancel()
	}
	job.Status = "cancelled"
	job.Stage = "cancelled"
	job.ETASeconds = 0
	job.Error = "cancelled"
	job.FinishedAt = time.Now().UTC()
	if err := r.store.UpdateJob(ctx, job); err != nil {
		return storage.Job{}, err
	}
	return job, nil
}
