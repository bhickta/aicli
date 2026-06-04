package server

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider/registry"
	"github.com/bhickta/aicli/internal/storage"
)

type memoryStore struct {
	jobs    map[string]storage.Job
	reviews map[string]storage.TopperReviewRecord
}

func (m *memoryStore) Migrate() error { return nil }
func (m *memoryStore) CreateJob(_ context.Context, job storage.Job) error {
	if m.jobs == nil {
		m.jobs = map[string]storage.Job{}
	}
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
	if m.jobs == nil {
		m.jobs = map[string]storage.Job{}
	}
	m.jobs[job.ID] = job
	return nil
}
func (m *memoryStore) DeleteJob(_ context.Context, id string) error {
	if _, ok := m.jobs[id]; !ok {
		return storage.ErrNotFound
	}
	delete(m.jobs, id)
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

func (m *memoryStore) SaveTopperReview(_ context.Context, record storage.TopperReviewRecord) error {
	if m.reviews == nil {
		m.reviews = map[string]storage.TopperReviewRecord{}
	}
	m.reviews[record.ID] = record
	return nil
}

func (m *memoryStore) DeleteTopperReview(_ context.Context, id string) error {
	if _, ok := m.reviews[id]; !ok {
		return storage.ErrNotFound
	}
	delete(m.reviews, id)
	return nil
}

func (m *memoryStore) GetTopperReview(_ context.Context, id string) (storage.TopperReviewRecord, error) {
	record, ok := m.reviews[id]
	if !ok {
		return storage.TopperReviewRecord{}, storage.ErrNotFound
	}
	return record, nil
}

func (m *memoryStore) ListTopperReviews(_ context.Context, opts storage.TopperReviewListOptions) ([]storage.TopperReviewRecord, error) {
	query := strings.ToLower(opts.Query)
	records := make([]storage.TopperReviewRecord, 0, len(m.reviews))
	for _, record := range m.reviews {
		if query != "" && !strings.Contains(strings.ToLower(record.PDFName+" "+record.SourcePath+" "+record.SearchText), query) {
			continue
		}
		record.ReviewJSON = ""
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	return records, nil
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
