package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestListFiles(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/list?path=.", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
}

func TestUploadFilesStoresDroppedFile(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "sample.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write([]byte("%PDF-1.7\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	handler := testHandlerWithDataDir(t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/fs/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Files []uploadEntry `json:"files"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0].Name != "sample.pdf" {
		t.Fatalf("name = %q, want sample.pdf", payload.Files[0].Name)
	}
	if _, err := os.Stat(payload.Files[0].Path); err != nil {
		t.Fatalf("uploaded file was not stored: %v", err)
	}
}

func testHandler() http.Handler {
	return testHandlerWithDataDir("")
}

func testHandlerWithDataDir(dataDir string) http.Handler {
	settings := config.DefaultSettings()
	return New(Dependencies{
		Logger:    slog.Default(),
		DataDir:   dataDir,
		Settings:  settings,
		Store:     &memoryStore{jobs: map[string]storage.Job{}},
		Providers: provider.NewRegistry(settings.Providers),
	})
}
