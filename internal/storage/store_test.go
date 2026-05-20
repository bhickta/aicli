package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
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
