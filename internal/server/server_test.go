package server

import (
	"context"
	"log/slog"
	"net/http"
	"sort"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider/registry"
	"github.com/bhickta/aicli/internal/storage"
)

type memoryStore struct {
	jobs map[string]storage.Job
}

func (m *memoryStore) Migrate() error { return nil }
func (m *memoryStore) CreateJob(_ context.Context, job storage.Job) error {
	m.jobs[job.ID] = job
	return nil
}
func (m *memoryStore) GetJob(_ context.Context, id string) (storage.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return storage.Job{}, storage.ErrNotFound
	}
	return job, nil
}
func (m *memoryStore) ListJobs(_ context.Context) ([]storage.Job, error) {
	jobs := make([]storage.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}
func (m *memoryStore) ListJobsFiltered(ctx context.Context, opts storage.JobListOptions) ([]storage.Job, error) {
	jobs, err := m.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	return filterJobs(jobs, opts), nil
}
func (m *memoryStore) UpdateJob(_ context.Context, job storage.Job) error {
	m.jobs[job.ID] = job
	return nil
}
func (m *memoryStore) DeleteFinishedJobs(_ context.Context) (int64, error) {
	var deleted int64
	for id, job := range m.jobs {
		if job.Status == storage.JobStatusCompleted || job.Status == storage.JobStatusFailed || job.Status == storage.JobStatusCancelled {
			delete(m.jobs, id)
			deleted++
		}
	}
	return deleted, nil
}

func testHandler() http.Handler {
	return testHandlerWithDataDir("")
}

func testHandlerWithDataDir(dataDir string) http.Handler {
	settings := config.DefaultSettings()
	return testHandlerWithSettings(settings, dataDir)
}

func testHandlerWithSettings(settings config.Settings, dataDir string) http.Handler {
	return testHandlerWithSettingsAndStore(settings, dataDir, &memoryStore{jobs: map[string]storage.Job{}})
}

func testHandlerWithSettingsAndStore(settings config.Settings, dataDir string, store storage.Store) http.Handler {
	return New(Dependencies{
		Logger:    slog.Default(),
		DataDir:   dataDir,
		Settings:  settings,
		Store:     store,
		Providers: registry.New(settings.Providers, settings.Tools),
	})
}
