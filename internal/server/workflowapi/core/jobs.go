package core

import (
	"fmt"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

func NewJob(jobType string, input string) storage.Job {
	now := time.Now().UTC()
	return storage.Job{
		ID:          fmt.Sprintf("%s-%d", jobType, now.UnixNano()),
		Type:        jobType,
		Status:      storage.JobStatusRunning,
		Stage:       "queued",
		Progress:    0,
		CurrentStep: 0,
		TotalSteps:  4,
		Input:       input,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
