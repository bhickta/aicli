package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

func (r *Runtime) finishJobStore(ctx context.Context, job storage.Job, result any, err error) {
	job.FinishedAt = time.Now().UTC()
	job.ETASeconds = 0
	if err != nil {
		r.finishFailedJob(ctx, job, err)
		return
	}

	output, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		job.Status = storage.JobStatusFailed
		job.Stage = storage.JobStatusFailed
		job.Error = marshalErr.Error()
		_ = r.store.UpdateJob(ctx, job)
		if r.logger != nil {
			r.logger.Error("workflow result marshal failed", "job", job.ID, "type", job.Type, "error", marshalErr)
		}
		return
	}

	job.Status = storage.JobStatusCompleted
	job.Stage = storage.JobStatusCompleted
	job.Progress = 1
	job.CurrentStep = job.TotalSteps
	job.Output = string(output)
	if err := r.store.UpdateJob(ctx, job); err != nil && r.logger != nil {
		r.logger.Error("workflow completion update failed", "job", job.ID, "type", job.Type, "error", err)
	}
	if r.logger != nil {
		r.logger.Info("workflow completed", "job", job.ID, "type", job.Type)
	}
}

func (r *Runtime) finishFailedJob(ctx context.Context, job storage.Job, err error) {
	if errors.Is(err, context.Canceled) {
		job.Status = storage.JobStatusCancelled
		job.Stage = storage.JobStatusCancelled
		job.Error = storage.JobStatusCancelled
		_ = r.store.UpdateJob(ctx, job)
		if r.logger != nil {
			r.logger.Info("workflow cancelled", "job", job.ID, "type", job.Type)
		}
		return
	}

	job.Status = storage.JobStatusFailed
	job.Stage = storage.JobStatusFailed
	job.Error = err.Error()
	_ = r.store.UpdateJob(ctx, job)
	if r.logger != nil {
		r.logger.Error("workflow failed", "job", job.ID, "type", job.Type, "error", err)
	}
}
