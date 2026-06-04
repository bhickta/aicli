package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/storage"
)

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

func TestListJobsDefaultsToRecentLimitAndSupportsStatusFilter(t *testing.T) {
	t.Parallel()

	store := &memoryStore{jobs: map[string]storage.Job{}}
	base := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	for i, job := range []storage.Job{
		{ID: "old-completed", Type: "ocr", Status: storage.JobStatusCompleted, CreatedAt: base.Add(-3 * time.Hour)},
		{ID: "new-failed", Type: "video", Status: storage.JobStatusFailed, CreatedAt: base.Add(-time.Hour)},
		{ID: "new-running", Type: "video", Status: storage.JobStatusRunning, CreatedAt: base},
	} {
		job.UpdatedAt = job.CreatedAt
		if err := store.CreateJob(context.Background(), job); err != nil {
			t.Fatalf("CreateJob(%d) error = %v", i, err)
		}
	}
	handler := testHandlerWithSettingsAndStore(config.DefaultSettings(), "", store)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=2", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Jobs []storage.Job `json:"jobs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Jobs) != 2 || payload.Jobs[0].ID != "new-running" || payload.Jobs[1].ID != "new-failed" {
		t.Fatalf("recent jobs = %#v, want newest two", payload.Jobs)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/jobs?status=running", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("running status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	payload = struct {
		Jobs []storage.Job `json:"jobs"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].ID != "new-running" {
		t.Fatalf("running jobs = %#v, want only new-running", payload.Jobs)
	}
}

func TestClearJobsDeletesFinishedOnly(t *testing.T) {
	t.Parallel()

	store := &memoryStore{jobs: map[string]storage.Job{}}
	for _, job := range []storage.Job{
		{ID: "running", Type: "video", Status: storage.JobStatusRunning},
		{ID: "completed", Type: "ocr", Status: storage.JobStatusCompleted},
		{ID: "failed", Type: "ocr", Status: storage.JobStatusFailed},
	} {
		if err := store.CreateJob(context.Background(), job); err != nil {
			t.Fatalf("CreateJob(%s) error = %v", job.ID, err)
		}
	}
	handler := testHandlerWithSettingsAndStore(config.DefaultSettings(), "", store)

	req := httptest.NewRequest(http.MethodDelete, "/api/jobs?scope=finished", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2", payload.Deleted)
	}
	if _, err := store.GetJob(context.Background(), "running"); err != nil {
		t.Fatalf("running job should remain: %v", err)
	}
	if _, err := store.GetJob(context.Background(), "completed"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("completed GetJob error = %v, want ErrNotFound", err)
	}
}

func TestTopperReviewArchiveEndpoints(t *testing.T) {
	t.Parallel()

	store := &memoryStore{jobs: map[string]storage.Job{}, reviews: map[string]storage.TopperReviewRecord{}}
	reviewJSON := `{"kind":"topper_copy_review","review_id":"topper-1","pdf_name":"copy.pdf","pages":[{"number":1,"name":"page-1","path":"","image_url":"","text":"governance answer","unclear_count":0,"verified":false}],"questions":[],"report":"good answer"}`
	if err := store.SaveTopperReview(context.Background(), storage.TopperReviewRecord{
		ID:         "topper-1",
		PDFName:    "copy.pdf",
		ReviewJSON: reviewJSON,
		SearchText: "governance answer",
		UpdatedAt:  time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	handler := testHandlerWithSettingsAndStore(config.DefaultSettings(), "", store)

	req := httptest.NewRequest(http.MethodGet, "/api/topper-reviews?query=governance", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var listPayload struct {
		Reviews []storage.TopperReviewRecord `json:"reviews"`
	}
	if err := json.NewDecoder(res.Body).Decode(&listPayload); err != nil {
		t.Fatal(err)
	}
	if len(listPayload.Reviews) != 1 || listPayload.Reviews[0].ID != "topper-1" {
		t.Fatalf("reviews = %#v, want topper-1", listPayload.Reviews)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/topper-reviews/topper-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200, body=%s", res.Code, res.Body.String())
	}

	updated := strings.NewReader(`{"kind":"topper_copy_review","review_id":"topper-1","pdf_name":"copy.pdf","pages":[],"questions":[],"report":"edited"}`)
	req = httptest.NewRequest(http.MethodPut, "/api/topper-reviews/topper-1", updated)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	record, err := store.GetTopperReview(context.Background(), "topper-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(record.ReviewJSON, "edited") || record.Status != "edited" {
		t.Fatalf("record after update = %#v, want edited review", record)
	}
}
