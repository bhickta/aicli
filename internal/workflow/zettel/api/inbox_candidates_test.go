package zettel

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceInboxCandidatePreviewReturnsEmbeddingSelections(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md"), "- **Inflation**: price rise.\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "polity.md"), "- **Parliament**: law making.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "inflation.md"), "- **Inflation Spike**: 7% due to oil.\n")

	provider := &fakeZettelProvider{}
	service := NewWithProviders(nil, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	options.CandidateLimit = 2
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxCandidatePreview(context.Background(), InboxCandidatePreviewRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxCandidatePreview() error = %v", err)
	}
	if resp.SourceCount != 1 || resp.SelectedCount != 1 || resp.SkippedCount != 0 {
		t.Fatalf("preview counts = %#v, want one selected source", resp)
	}
	if len(resp.Sources) != 1 {
		t.Fatalf("sources = %d, want one", len(resp.Sources))
	}
	source := resp.Sources[0]
	if source.SourcePath != "in/inflation.md" || !strings.Contains(source.SourceExcerpt, "Inflation Spike") {
		t.Fatalf("source preview = %#v, want source path and excerpt", source)
	}
	if len(source.Candidates) != 2 {
		t.Fatalf("candidates = %#v, want two embedding matches", source.Candidates)
	}
	if source.Candidates[0].Path == "" || source.Candidates[0].Similarity <= 0 || source.Candidates[0].Excerpt == "" {
		t.Fatalf("top candidate = %#v, want path, score, and excerpt", source.Candidates[0])
	}
	if len(provider.chatCalls) != 0 {
		t.Fatalf("chat calls = %d, want preview to avoid merge model", len(provider.chatCalls))
	}
	if resp.APICalls.Chat != 0 || resp.APICalls.Embeddings == 0 {
		t.Fatalf("api calls = %#v, want embedding-only preview", resp.APICalls)
	}
}

func TestServiceInboxCandidatePreviewRespectsInboxLimit(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Seed**: candidate.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "a.md"), "- **A**: one.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "b.md"), "- **B**: two.\n")

	provider := &fakeZettelProvider{}
	service := NewWithProviders(nil, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	options.InboxLimit = 1
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxCandidatePreview(context.Background(), InboxCandidatePreviewRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxCandidatePreview() error = %v", err)
	}
	if resp.SourceCount != 2 || resp.SelectedCount != 1 || resp.SkippedCount != 1 || resp.Limit != 1 {
		t.Fatalf("preview counts = %#v, want one selected and one skipped", resp)
	}
	if got := resp.Sources[0].SourcePath; got != "in/a.md" {
		t.Fatalf("selected source = %q, want first sorted inbox note", got)
	}
}
