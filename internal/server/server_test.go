package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
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

func TestHealth(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
}

func TestCreateAndGetJob(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	body := strings.NewReader(`{"id":"job-1","type":"ocr"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/jobs/job-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", res.Code)
	}
	var job storage.Job
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" {
		t.Fatalf("Status = %q, want queued", job.Status)
	}
}

func testHandler() http.Handler {
	settings := config.DefaultSettings()
	return New(Dependencies{
		Logger:    slog.Default(),
		Settings:  settings,
		Store:     &memoryStore{jobs: map[string]storage.Job{}},
		Providers: provider.NewRegistry(settings.Providers),
	})
}
