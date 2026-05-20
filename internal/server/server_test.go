package server

import (
	"context"
	"log/slog"
	"net/http"

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
	return jobs, nil
}
func (m *memoryStore) UpdateJob(_ context.Context, job storage.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func testHandler() http.Handler {
	return testHandlerWithDataDir("")
}

func testHandlerWithDataDir(dataDir string) http.Handler {
	settings := config.DefaultSettings()
	return testHandlerWithSettings(settings, dataDir)
}

func testHandlerWithSettings(settings config.Settings, dataDir string) http.Handler {
	return New(Dependencies{
		Logger:    slog.Default(),
		DataDir:   dataDir,
		Settings:  settings,
		Store:     &memoryStore{jobs: map[string]storage.Job{}},
		Providers: registry.New(settings.Providers, settings.Tools),
	})
}
