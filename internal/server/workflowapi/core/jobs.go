package core

import (
	"fmt"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/storage"
)

func NewJob(jobType string, input string) storage.Job {
	now := time.Now().UTC()
	return storage.Job{
		ID:           fmt.Sprintf("%s-%d", jobType, now.UnixNano()),
		Type:         jobType,
		Status:       storage.JobStatusRunning,
		Stage:        "queued",
		Progress:     0,
		CurrentStep:  0,
		TotalSteps:   0,
		ProgressMode: progressmodel.ModeIndeterminate,
		Input:        input,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
