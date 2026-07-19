package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/execution"
)

func TestExecutionUsagePersistsAcrossStoreInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aicli.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	event := execution.UsageEvent{
		ProfileID: "structured", ProviderID: "gemini-key-1", Model: "flash-lite",
		OccurredAt: now, ReservedTokens: 1234,
	}
	if err := store.RecordExecutionUsage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store = NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	events, err := store.LoadExecutionUsage(context.Background(), now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ProviderID != event.ProviderID || events[0].ReservedTokens != event.ReservedTokens {
		t.Fatalf("events = %#v, want persisted event %#v", events, event)
	}
}
