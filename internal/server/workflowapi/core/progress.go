package core

import (
	"context"
	"net/http"

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
		storeCtx := context.Background()
		runCtx, cancel := context.WithCancel(context.Background())
		r.registerCancel(job.ID, cancel)
		defer r.unregisterCancel(job.ID)
		defer cancel()
		r.updateJobProgress(storeCtx, &job, "started", 1, 4)
		result, err := run(runCtx, func(stage string, currentStep, totalSteps int) {
			r.updateJobProgress(storeCtx, &job, stage, currentStep, totalSteps)
		})
		r.finishJobStore(storeCtx, job, result, err)
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
