package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/storage"
)

func TestNewMarksPersistedRunningJobsInterrupted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "aicli.db")
	db, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateJob(context.Background(), storage.Job{ID: "running", Type: "video-course", Status: storage.JobStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	application, err := New(Options{DataDir: dir, ConfigPath: filepath.Join(dir, "settings.json")}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store = storage.NewSQLiteStore(db)
	job, err := store.GetJob(context.Background(), "running")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != storage.JobStatusFailed || job.Stage != "interrupted" || job.Error != "interrupted by AICLI restart" {
		t.Fatalf("job after app restart = %#v", job)
	}
}
