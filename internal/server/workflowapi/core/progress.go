package core

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

type ProgressFunc = func(stage string, currentStep, totalSteps int)

func (r *Runtime) StartWorkflow(w http.ResponseWriter, req *http.Request, job storage.Job, run func(context.Context, ProgressFunc) (any, error)) {
	if err := r.store.CreateJob(req.Context(), job); err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, map[string]any{"job": job})

	go func() {
		ctx := context.Background()
		r.updateJobProgress(ctx, &job, "started", 1, 4)
		result, err := run(ctx, func(stage string, currentStep, totalSteps int) {
			r.updateJobProgress(ctx, &job, stage, currentStep, totalSteps)
		})
		r.finishJobStore(ctx, job, result, err)
	}()
}

func (r *Runtime) StartJob(w http.ResponseWriter, req *http.Request, jobType string, input string, run func(context.Context, ProgressFunc) (any, error)) {
	r.StartWorkflow(w, req, NewJob(jobType, input), run)
}

func (r *Runtime) updateJobProgress(ctx context.Context, job *storage.Job, stage string, currentStep, totalSteps int) {
	if totalSteps <= 0 {
		totalSteps = job.TotalSteps
	}
	if totalSteps <= 0 {
		totalSteps = 1
	}
	if currentStep < 0 {
		currentStep = 0
	}
	if currentStep > totalSteps {
		currentStep = totalSteps
	}
	job.Stage = stage
	job.CurrentStep = currentStep
	job.TotalSteps = totalSteps
	job.Progress = float64(currentStep) / float64(totalSteps)
	job.ETASeconds = estimateETA(*job)
	if err := r.store.UpdateJob(ctx, *job); err != nil && r.logger != nil {
		r.logger.Warn("job progress update failed", "job", job.ID, "stage", stage, "error", err)
	}
	if r.logger != nil {
		r.logger.Info("workflow progress", "job", job.ID, "type", job.Type, "stage", stage, "step", currentStep, "total", totalSteps, "eta_seconds", job.ETASeconds)
	}
}

func (r *Runtime) finishJobStore(ctx context.Context, job storage.Job, result any, err error) {
	job.FinishedAt = time.Now().UTC()
	job.ETASeconds = 0
	if err != nil {
		job.Status = "failed"
		job.Stage = "failed"
		job.Error = err.Error()
		_ = r.store.UpdateJob(ctx, job)
		if r.logger != nil {
			r.logger.Error("workflow failed", "job", job.ID, "type", job.Type, "error", err)
		}
		return
	}
	output, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		job.Status = "failed"
		job.Stage = "failed"
		job.Error = marshalErr.Error()
		_ = r.store.UpdateJob(ctx, job)
		if r.logger != nil {
			r.logger.Error("workflow result marshal failed", "job", job.ID, "type", job.Type, "error", marshalErr)
		}
		return
	}
	job.Status = "completed"
	job.Stage = "completed"
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

func estimateETA(job storage.Job) int {
	if job.Progress <= 0 || job.Progress >= 1 || job.CreatedAt.IsZero() {
		return 0
	}
	elapsed := time.Since(job.CreatedAt)
	if elapsed < 30*time.Second || job.CurrentStep < 1 || job.Progress < 0.05 {
		return 0
	}
	total := time.Duration(float64(elapsed) / job.Progress)
	remaining := total - elapsed
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Round(time.Second).Seconds())
}
