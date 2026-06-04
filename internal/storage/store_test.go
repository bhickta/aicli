package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreJobLifecycle(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	job := Job{ID: "job-1", Type: "ocr", Status: "queued", Input: "{}"}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	got, err := store.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %q, want queued", got.Status)
	}

	got.Status = "completed"
	got.Output = "done"
	if err := store.UpdateJob(context.Background(), got); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}

	jobs, err := store.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != "completed" {
		t.Fatalf("jobs = %#v, want one completed job", jobs)
	}
}

func TestSQLiteStoreGetJobNotFound(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	_, err = store.GetJob(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetJob() error = %v, want ErrNotFound", err)
	}
}

func TestSQLiteStoreListJobsFiltered(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	for i, job := range []Job{
		{ID: "old", Type: "ocr", Status: JobStatusCompleted, CreatedAt: base.Add(-2 * time.Hour)},
		{ID: "failed", Type: "video", Status: JobStatusFailed, CreatedAt: base.Add(-time.Hour)},
		{ID: "running", Type: "video", Status: JobStatusRunning, CreatedAt: base},
	} {
		job.UpdatedAt = job.CreatedAt
		if err := store.CreateJob(context.Background(), job); err != nil {
			t.Fatalf("CreateJob(%d) error = %v", i, err)
		}
	}

	recent, err := store.ListJobsFiltered(context.Background(), JobListOptions{Status: "recent", Limit: 2})
	if err != nil {
		t.Fatalf("ListJobsFiltered(recent) error = %v", err)
	}
	if len(recent) != 2 || recent[0].ID != "running" || recent[1].ID != "failed" {
		t.Fatalf("recent jobs = %#v, want running then failed", recent)
	}

	running, err := store.ListJobsFiltered(context.Background(), JobListOptions{Status: JobStatusRunning, Limit: 20})
	if err != nil {
		t.Fatalf("ListJobsFiltered(running) error = %v", err)
	}
	if len(running) != 1 || running[0].ID != "running" {
		t.Fatalf("running jobs = %#v, want only running", running)
	}
}

func TestSQLiteStoreDeletesFinishedAndMarksRunningInterrupted(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, job := range []Job{
		{ID: "running", Type: "video", Status: JobStatusRunning},
		{ID: "completed", Type: "ocr", Status: JobStatusCompleted},
		{ID: "cancelled", Type: "ocr", Status: JobStatusCancelled},
	} {
		if err := store.CreateJob(context.Background(), job); err != nil {
			t.Fatalf("CreateJob(%s) error = %v", job.ID, err)
		}
	}

	interrupted, err := store.MarkRunningJobsInterrupted(context.Background(), "interrupted by restart")
	if err != nil {
		t.Fatalf("MarkRunningJobsInterrupted() error = %v", err)
	}
	if interrupted != 1 {
		t.Fatalf("interrupted = %d, want 1", interrupted)
	}
	running, err := store.GetJob(context.Background(), "running")
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != JobStatusFailed || running.Stage != "interrupted" || running.Error != "interrupted by restart" || running.FinishedAt.IsZero() {
		t.Fatalf("running job after interrupt = %#v", running)
	}

	deleted, err := store.DeleteFinishedJobs(context.Background())
	if err != nil {
		t.Fatalf("DeleteFinishedJobs() error = %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
	jobs, err := store.ListJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs after delete = %#v, want none", jobs)
	}
}

func TestSQLiteStoreTopperReviewLifecycle(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	review := TopperReviewRecord{
		ID:            "topper-1",
		JobID:         "job-1",
		PDFName:       "essay.pdf",
		SourcePath:    "/tmp/essay.pdf",
		ProviderID:    "lms",
		Model:         "vision",
		PageCount:     2,
		QuestionCount: 3,
		UnclearCount:  1,
		Status:        "ready",
		ReviewJSON:    `{"kind":"topper_copy_review","review_id":"topper-1"}`,
		SearchText:    "essay governance ethics",
	}
	if err := store.SaveTopperReview(context.Background(), review); err != nil {
		t.Fatalf("SaveTopperReview() error = %v", err)
	}

	got, err := store.GetTopperReview(context.Background(), "topper-1")
	if err != nil {
		t.Fatalf("GetTopperReview() error = %v", err)
	}
	if got.PDFName != "essay.pdf" || got.QuestionCount != 3 || got.ReviewJSON == "" {
		t.Fatalf("record = %#v, want saved review", got)
	}

	matches, err := store.ListTopperReviews(context.Background(), TopperReviewListOptions{Query: "ethics"})
	if err != nil {
		t.Fatalf("ListTopperReviews() error = %v", err)
	}
	if len(matches) != 1 || matches[0].ID != "topper-1" || matches[0].ReviewJSON != "" {
		t.Fatalf("matches = %#v, want summary without JSON payload", matches)
	}

	review.QuestionCount = 4
	review.Status = "edited"
	if err := store.SaveTopperReview(context.Background(), review); err != nil {
		t.Fatalf("SaveTopperReview(update) error = %v", err)
	}
	got, err = store.GetTopperReview(context.Background(), "topper-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.QuestionCount != 4 || got.Status != "edited" {
		t.Fatalf("updated record = %#v, want edited count", got)
	}
}

func TestOpenSQLiteConfiguresLockResistantConnection(t *testing.T) {
	t.Parallel()

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "aicli.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}
	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 10000 {
		t.Fatalf("busy_timeout = %d, want 10000", busyTimeout)
	}
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}
