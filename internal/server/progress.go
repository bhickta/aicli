package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

type progressFunc = func(stage string, currentStep, totalSteps int)

func (s *Server) startWorkflow(w http.ResponseWriter, r *http.Request, job storage.Job, run func(context.Context, progressFunc) (any, error)) {
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})

	go func() {
		ctx := context.Background()
		s.updateJobProgress(ctx, &job, "started", 1, 4)
		result, err := run(ctx, func(stage string, currentStep, totalSteps int) {
			s.updateJobProgress(ctx, &job, stage, currentStep, totalSteps)
		})
		s.finishJobStore(ctx, job, result, err)
	}()
}

func (s *Server) updateJobProgress(ctx context.Context, job *storage.Job, stage string, currentStep, totalSteps int) {
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
	if err := s.deps.Store.UpdateJob(ctx, *job); err != nil && s.deps.Logger != nil {
		s.deps.Logger.Warn("job progress update failed", "job", job.ID, "stage", stage, "error", err)
	}
	if s.deps.Logger != nil {
		s.deps.Logger.Info("workflow progress", "job", job.ID, "type", job.Type, "stage", stage, "step", currentStep, "total", totalSteps, "eta_seconds", job.ETASeconds)
	}
}

func (s *Server) finishJobStore(ctx context.Context, job storage.Job, result any, err error) {
	job.FinishedAt = time.Now().UTC()
	job.ETASeconds = 0
	if err != nil {
		job.Status = "failed"
		job.Stage = "failed"
		job.Error = err.Error()
		_ = s.deps.Store.UpdateJob(ctx, job)
		if s.deps.Logger != nil {
			s.deps.Logger.Error("workflow failed", "job", job.ID, "type", job.Type, "error", err)
		}
		return
	}
	output, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		job.Status = "failed"
		job.Stage = "failed"
		job.Error = marshalErr.Error()
		_ = s.deps.Store.UpdateJob(ctx, job)
		if s.deps.Logger != nil {
			s.deps.Logger.Error("workflow result marshal failed", "job", job.ID, "type", job.Type, "error", marshalErr)
		}
		return
	}
	job.Status = "completed"
	job.Stage = "completed"
	job.Progress = 1
	job.CurrentStep = job.TotalSteps
	job.Output = string(output)
	if err := s.deps.Store.UpdateJob(ctx, job); err != nil && s.deps.Logger != nil {
		s.deps.Logger.Error("workflow completion update failed", "job", job.ID, "type", job.Type, "error", err)
	}
	if s.deps.Logger != nil {
		s.deps.Logger.Info("workflow completed", "job", job.ID, "type", job.Type)
	}
}

func estimateETA(job storage.Job) int {
	if job.Progress <= 0 || job.Progress >= 1 || job.CreatedAt.IsZero() {
		return 0
	}
	elapsed := time.Since(job.CreatedAt)
	total := time.Duration(float64(elapsed) / job.Progress)
	remaining := total - elapsed
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Round(time.Second).Seconds())
}
