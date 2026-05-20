package core

import (
	"testing"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/storage"
)

func TestApplyProgressUpdateIndeterminate(t *testing.T) {
	t.Parallel()

	job := storage.Job{Progress: 0.75, CurrentStep: 3, TotalSteps: 4}
	applyProgressUpdate(&job, progressmodel.Indeterminate("calling model"), time.Now())

	if job.ProgressMode != progressmodel.ModeIndeterminate {
		t.Fatalf("ProgressMode = %q, want indeterminate", job.ProgressMode)
	}
	if job.Progress != 0 || job.CurrentStep != 0 || job.TotalSteps != 0 {
		t.Fatalf("job progress = %v/%d/%d, want zeroed unknown progress", job.Progress, job.CurrentStep, job.TotalSteps)
	}
}

func TestApplyProgressUpdateDeterminateClampsUnits(t *testing.T) {
	t.Parallel()

	job := storage.Job{}
	applyProgressUpdate(&job, progressmodel.Units("compressing", 12, 10, "video"), time.Now())

	if job.ProgressMode != progressmodel.ModeDeterminate {
		t.Fatalf("ProgressMode = %q, want determinate", job.ProgressMode)
	}
	if job.CompletedUnits != 10 || job.TotalUnits != 10 || job.Progress != 1 {
		t.Fatalf("job progress = %d/%d %.2f, want 10/10 1.0", job.CompletedUnits, job.TotalUnits, job.Progress)
	}
}

func TestApplyProgressUpdateTimedUsesWindow(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	endsAt := startedAt.Add(10 * time.Minute)
	now := startedAt.Add(3 * time.Minute)
	job := storage.Job{}
	applyProgressUpdate(&job, progressmodel.Timed("waiting", startedAt, endsAt), now)

	if job.ProgressMode != progressmodel.ModeTimed {
		t.Fatalf("ProgressMode = %q, want timed", job.ProgressMode)
	}
	if job.Progress < 0.29 || job.Progress > 0.31 {
		t.Fatalf("Progress = %.3f, want about 0.3", job.Progress)
	}
	if !job.ProgressStartedAt.Equal(startedAt) || !job.ProgressEndsAt.Equal(endsAt) {
		t.Fatalf("timed window = %s -> %s, want %s -> %s", job.ProgressStartedAt, job.ProgressEndsAt, startedAt, endsAt)
	}
}
