package zettel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceInboxMergeProcessesSourceAndRollbackRestores(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md"), "- **Inflation**:: 6%\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "batch", "inflation.md"), "Inflation rose to 7% due to oil prices.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Inflation = 7% due to oil prices","source":"Inflation rose to 7% due to oil prices."}],"notes":"ok"}`,
		`{"destinations":[{"path":"zettelkasten/economy.md","claim_ids":["c1"],"confidence":0.99,"reason":"inflation destination"}],"pending":[],"notes":"ok"}`,
		`{"final_markdown":"- **Inflation**:: 6%\n- **Inflation**:: 7% (Oil prices ^)","ledger":[{"claim_id":"c1","status":"merged","destination_path":"zettelkasten/economy.md","evidence":"added 7% oil price line","reason":"new fact"}],"notes":"ok"}`,
		`{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"ok"}`,
	}}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "inbox-to-merge",
		CandidateProviderID:  "fake",
		MergeProviderID:      "fake",
		ValidationProviderID: "fake",
		EmbeddingProviderID:  "fake",
		CandidateModel:       "judge-model",
		MergeModel:           "merge-model",
		ValidationModel:      "validation-model",
		EmbeddingModel:       "embedding-model",
	}
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 1 || resp.PendingCount != 0 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want one processed note", resp)
	}
	processed := resp.Processed[0]
	if processed.ProcessedPath == "" || !strings.HasPrefix(processed.ProcessedPath, "inbox-to-merge/_processed/") {
		t.Fatalf("processed path = %q, want processed folder path", processed.ProcessedPath)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "inbox-to-merge", "batch", "inflation.md")); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md")); !strings.Contains(got, "7% (Oil prices ^)") {
		t.Fatalf("destination content = %q, want shorthand merged fact", got)
	}
	if _, err := os.Stat(filepath.Join(resp.ArchivePath, "manifest.json")); err != nil {
		t.Fatalf("inbox manifest missing: %v", err)
	}

	rollback, err := service.Rollback(context.Background(), RollbackRequest{Options: options, JobID: resp.RunID}, nil)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rollback.JobID != resp.RunID {
		t.Fatalf("rollback job id = %q, want %q", rollback.JobID, resp.RunID)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md")); got != "- **Inflation**:: 6%\n" {
		t.Fatalf("restored destination = %q", got)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "batch", "inflation.md")); got != "Inflation rose to 7% due to oil prices.\n" {
		t.Fatalf("restored source = %q", got)
	}
}

func TestServiceInboxMergeKeepsPendingSourceUnchanged(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md"), "- **Inflation**:: 6%\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "stray.md"), "Ambiguous note with no safe destination.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Ambiguous note with no safe destination","source":"Ambiguous note with no safe destination."}],"notes":"ok"}`,
		`{"destinations":[],"pending":[{"claim_id":"c1","status":"pending","reason":"no confident destination"}],"notes":"pending"}`,
	}}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "inbox-to-merge",
		CandidateProviderID:  "fake",
		MergeProviderID:      "fake",
		ValidationProviderID: "fake",
		EmbeddingProviderID:  "fake",
		CandidateModel:       "judge-model",
		MergeModel:           "merge-model",
		ValidationModel:      "validation-model",
		EmbeddingModel:       "embedding-model",
	}
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.PendingCount != 1 || resp.ProcessedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want one pending note", resp)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "stray.md")); got != "Ambiguous note with no safe destination.\n" {
		t.Fatalf("pending source changed = %q", got)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md")); got != "- **Inflation**:: 6%\n" {
		t.Fatalf("destination changed for pending note = %q", got)
	}
	if resp.Pending[0].Reason != "no confident destination" {
		t.Fatalf("pending reason = %q, want judge reason", resp.Pending[0].Reason)
	}
}

func TestServiceInboxMergeRespectsInboxLimit(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "001.md"), "first\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "002.md"), "second\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "003.md"), "third\n")

	provider := &fakeZettelProvider{}
	service := NewWithProviders(provider, provider, provider, provider)
	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "inbox-to-merge",
		InboxLimit:           2,
		CandidateProviderID:  "fake",
		MergeProviderID:      "fake",
		ValidationProviderID: "fake",
		EmbeddingProviderID:  "fake",
		CandidateModel:       "judge-model",
		MergeModel:           "merge-model",
		ValidationModel:      "validation-model",
		EmbeddingModel:       "embedding-model",
	}}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.SourceCount != 3 || resp.SelectedCount != 2 || resp.SkippedCount != 1 || resp.Limit != 2 {
		t.Fatalf("InboxMerge() = %#v, want two selected out of three", resp)
	}
	if resp.FailedCount != 2 {
		t.Fatalf("failed count = %d, want only selected notes attempted", resp.FailedCount)
	}
	if len(resp.Failed) != 2 || resp.Failed[0].SourcePath != "inbox-to-merge/001.md" || resp.Failed[1].SourcePath != "inbox-to-merge/002.md" {
		t.Fatalf("failed paths = %#v, want sorted first two notes only", resp.Failed)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "inbox-to-merge", "003.md")); err != nil {
		t.Fatalf("limited-out source changed: %v", err)
	}
}
