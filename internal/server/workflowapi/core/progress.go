package core

import (
	"context"
	"net/http"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/storage"
)

type ProgressFunc = progressmodel.Func
type ProgressUpdate = progressmodel.Update

func Indeterminate(stage string) ProgressUpdate {
	return progressmodel.Indeterminate(stage)
}

func Units(stage string, completed int, total int, label string) ProgressUpdate {
	return progressmodel.Units(stage, completed, total, label)
}

func Timed(stage string, startedAt time.Time, endsAt time.Time) ProgressUpdate {
	return progressmodel.Timed(stage, startedAt, endsAt)
}

func Step(stage string, currentStep int, totalSteps int) ProgressUpdate {
	return progressmodel.Step(stage, currentStep, totalSteps)
}

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
		r.updateJobProgress(storeCtx, &job, progressmodel.Indeterminate("started"))
		result, err := run(runCtx, func(update progressmodel.Update) {
			r.updateJobProgress(storeCtx, &job, update)
		})
		r.finishJobStore(storeCtx, job, result, err)
	}()
}

func (r *Runtime) StartJob(w http.ResponseWriter, req *http.Request, jobType string, input string, run func(context.Context, ProgressFunc) (any, error)) {
	r.StartWorkflow(w, req, NewJob(jobType, input), run)
}

func (r *Runtime) updateJobProgress(ctx context.Context, job *storage.Job, update progressmodel.Update) {
	applyProgressUpdate(job, update, time.Now().UTC())
	job.ETASeconds = estimateETA(*job)
	if err := r.store.UpdateJob(ctx, *job); err != nil && r.logger != nil {
		r.logger.Warn("job progress update failed", "job", job.ID, "stage", job.Stage, "error", err)
	}
	if r.logger != nil {
		r.logger.Info(
			"workflow progress",
			"job", job.ID,
			"type", job.Type,
			"stage", job.Stage,
			"mode", job.ProgressMode,
			"completed_units", job.CompletedUnits,
			"total_units", job.TotalUnits,
			"unit_label", job.UnitLabel,
			"progress", job.Progress,
			"eta_seconds", job.ETASeconds,
		)
	}
}

func applyProgressUpdate(job *storage.Job, update progressmodel.Update, now time.Time) {
	if update.Stage != "" {
		job.Stage = update.Stage
	}
	switch update.Mode {
	case progressmodel.ModeDeterminate:
		applyDeterminateProgress(job, update)
	case progressmodel.ModeTimed:
		applyTimedProgress(job, update, now)
	default:
		applyIndeterminateProgress(job, update)
	}
}

func applyDeterminateProgress(job *storage.Job, update progressmodel.Update) {
	total := update.TotalUnits
	completed := update.CompletedUnits
	if total <= 0 {
		applyIndeterminateProgress(job, update)
		return
	}
	if completed < 0 {
		completed = 0
	}
	if completed > total {
		completed = total
	}
	job.ProgressMode = progressmodel.ModeDeterminate
	job.CompletedUnits = completed
	job.TotalUnits = total
	job.UnitLabel = update.UnitLabel
	job.ProgressStartedAt = time.Time{}
	job.ProgressEndsAt = time.Time{}
	job.CurrentStep = completed
	job.TotalSteps = total
	job.Progress = float64(completed) / float64(total)
}

func applyTimedProgress(job *storage.Job, update progressmodel.Update, now time.Time) {
	startedAt := update.StartedAt
	if startedAt.IsZero() {
		startedAt = job.CreatedAt
	}
	if startedAt.IsZero() {
		startedAt = now
	}
	if update.EndsAt.IsZero() || !update.EndsAt.After(startedAt) {
		applyIndeterminateProgress(job, update)
		return
	}
	progress := now.Sub(startedAt).Seconds() / update.EndsAt.Sub(startedAt).Seconds()
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	job.ProgressMode = progressmodel.ModeTimed
	job.CompletedUnits = 0
	job.TotalUnits = 0
	job.UnitLabel = "time"
	job.ProgressStartedAt = startedAt
	job.ProgressEndsAt = update.EndsAt
	job.CurrentStep = 0
	job.TotalSteps = 0
	job.Progress = progress
}

func applyIndeterminateProgress(job *storage.Job, update progressmodel.Update) {
	job.ProgressMode = progressmodel.ModeIndeterminate
	job.CompletedUnits = 0
	job.TotalUnits = 0
	job.UnitLabel = ""
	job.ProgressStartedAt = time.Time{}
	job.ProgressEndsAt = time.Time{}
	job.CurrentStep = 0
	job.TotalSteps = 0
	job.Progress = 0
}
