package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
)

type cancelTestStore struct {
	mu   sync.Mutex
	jobs map[string]storage.Job
}

func (s *cancelTestStore) Migrate() error { return nil }

func (s *cancelTestStore) CreateJob(_ context.Context, job storage.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *cancelTestStore) GetJob(_ context.Context, id string) (storage.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return storage.Job{}, storage.ErrNotFound
	}
	return job, nil
}

func (s *cancelTestStore) ListJobs(context.Context) ([]storage.Job, error) {
	return []storage.Job{}, nil
}

func (s *cancelTestStore) UpdateJob(_ context.Context, job storage.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func TestCancelJobCancelsRunningWorkflow(t *testing.T) {
	t.Parallel()

	store := &cancelTestStore{jobs: map[string]storage.Job{}}
	runtime := New(Dependencies{
		Store:    store,
		Settings: func() config.Settings { return config.DefaultSettings() },
		ProviderFor: func(string) (provider.Provider, bool) {
			return nil, false
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	res := httptest.NewRecorder()
	started := make(chan struct{})
	runtime.StartJob(res, req, "test-cancel", "input", func(ctx context.Context, progress ProgressFunc) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if res.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Job storage.Job `json:"job"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	<-started

	cancelled, err := runtime.CancelJob(context.Background(), payload.Job.ID)
	if err != nil {
		t.Fatalf("CancelJob() error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Status)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), payload.Job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "cancelled" && job.FinishedAt.IsZero() == false {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("workflow did not finish as cancelled")
}
