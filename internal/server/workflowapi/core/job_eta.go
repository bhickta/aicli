package core

import (
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

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
