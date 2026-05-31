package zettel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceMetadataWritesFrontmatterAndArchive(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	notePath := filepath.Join(vaultDir, "zettelkasten", "economy.md")
	writeTestFile(t, notePath, "---\nStatus: Read\naliases:\n  - Econ\n---\n# Economy\nGDP and inflation basics.\n")

	provider := &fakeZettelProvider{chatResponse: `{
		"title": "Economy, GDP, and Inflation Basics",
		"summary_keywords": "Economy, GDP, inflation, basics",
		"recall_questions": [
			"What are the core economy concepts in this note?",
			"How do GDP and inflation relate to the note?",
			"What basics must be recalled from this note?"
		]
	}`}
	options := metadataTestOptions(vaultDir)
	resp, err := NewWithProviders(provider, provider).Metadata(context.Background(), MetadataRequest{
		Options:        options,
		MetadataFolder: "zettelkasten",
		MetadataLimit:  1,
	}, nil)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if resp.ProcessedCount != 1 || resp.FailedCount != 0 || len(resp.Skipped) != 0 {
		t.Fatalf("Metadata() = %#v, want one processed note", resp)
	}
	if resp.APICalls.Total != 1 || resp.APICalls.Chat != 1 {
		t.Fatalf("api calls = %#v, want one chat call", resp.APICalls)
	}

	got := readTestFile(t, notePath)
	for _, want := range []string{
		"Status: Read",
		"aliases:",
		`title: "Economy, GDP, and Inflation Basics"`,
		`summary_keywords: "Economy, GDP, inflation, basics"`,
		`  - "What are the core economy concepts in this note?"`,
		"# Economy\nGDP and inflation basics.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("metadata note missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(resp.ArchivePath, "manifest.json")); err != nil {
		t.Fatalf("metadata manifest missing: %v", err)
	}
	llmArchives, err := filepath.Glob(filepath.Join(resp.ArchivePath, "llm", "*.json"))
	if err != nil {
		t.Fatalf("glob llm archives: %v", err)
	}
	if len(llmArchives) != 1 {
		t.Fatalf("llm archives = %v, want one saved metadata request/response", llmArchives)
	}
	training, err := filepath.Glob(filepath.Join(resp.ArchivePath, "training", "zettel-metadata-chat.jsonl"))
	if err != nil {
		t.Fatalf("glob training archive: %v", err)
	}
	if len(training) != 1 {
		t.Fatalf("training archives = %v, want one jsonl file", training)
	}
}

func TestServiceMetadataSkipsCompleteMetadata(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	notePath := filepath.Join(vaultDir, "zettelkasten", "ready.md")
	content := strings.Join([]string{
		"---",
		`title: "Ready Note"`,
		`summary_keywords: "ready, metadata"`,
		"recall_questions:",
		`  - "Question one?"`,
		`  - "Question two?"`,
		`  - "Question three?"`,
		"---",
		"Body.",
		"",
	}, "\n")
	writeTestFile(t, notePath, content)

	provider := &fakeZettelProvider{}
	resp, err := NewWithProviders(provider, provider).Metadata(context.Background(), MetadataRequest{
		Options:        metadataTestOptions(vaultDir),
		MetadataFolder: "zettelkasten",
	}, nil)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if resp.ProcessedCount != 0 || len(resp.Skipped) != 1 || resp.FailedCount != 0 {
		t.Fatalf("Metadata() = %#v, want one skipped note", resp)
	}
	if got := readTestFile(t, notePath); got != content {
		t.Fatalf("skipped note changed = %q", got)
	}
	if len(provider.chatCalls) != 0 {
		t.Fatalf("chat calls = %d, want no calls for complete metadata", len(provider.chatCalls))
	}
}

func TestServiceMetadataOverwriteReplacesOnlyMetadataFields(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	notePath := filepath.Join(vaultDir, "zettelkasten", "overwrite.md")
	writeTestFile(t, notePath, strings.Join([]string{
		"---",
		"Status: Read",
		`title: "Old Title"`,
		`summary_keywords: "old"`,
		"recall_questions:",
		`  - "Old one?"`,
		`  - "Old two?"`,
		`  - "Old three?"`,
		"tags:",
		"  - economy",
		"---",
		"- Body stays.",
		"",
	}, "\n"))

	provider := &fakeZettelProvider{chatResponse: `{
		"title": "New Detailed Title",
		"summary_keywords": "new, economy, body",
		"recall_questions": [
			"What is the note about?",
			"Which economy details matter?",
			"What body facts should be recalled?"
		]
	}`}
	resp, err := NewWithProviders(provider, provider).Metadata(context.Background(), MetadataRequest{
		Options:           metadataTestOptions(vaultDir),
		MetadataFolder:    "zettelkasten",
		MetadataOverwrite: true,
	}, nil)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if resp.ProcessedCount != 1 {
		t.Fatalf("Metadata() = %#v, want one overwritten note", resp)
	}

	got := readTestFile(t, notePath)
	if strings.Contains(got, "Old Title") || strings.Contains(got, "Old one?") {
		t.Fatalf("old metadata was not replaced:\n%s", got)
	}
	for _, want := range []string{
		"Status: Read",
		"tags:",
		"  - economy",
		`title: "New Detailed Title"`,
		"- Body stays.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("overwritten note missing %q:\n%s", want, got)
		}
	}
}

func TestServiceMetadataInvalidJSONMarksNoteFailed(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	notePath := filepath.Join(vaultDir, "zettelkasten", "bad.md")
	original := "# Bad\nContent.\n"
	writeTestFile(t, notePath, original)

	provider := &fakeZettelProvider{chatResponse: "not json"}
	resp, err := NewWithProviders(provider, provider).Metadata(context.Background(), MetadataRequest{
		Options:        metadataTestOptions(vaultDir),
		MetadataFolder: "zettelkasten",
	}, nil)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if resp.ProcessedCount != 0 || resp.FailedCount != 1 {
		t.Fatalf("Metadata() = %#v, want one failed note", resp)
	}
	if got := readTestFile(t, notePath); got != original {
		t.Fatalf("failed metadata changed note = %q", got)
	}
	manifestData, err := os.ReadFile(filepath.Join(resp.ArchivePath, "manifest.json"))
	if err != nil {
		t.Fatalf("read metadata manifest: %v", err)
	}
	var manifest struct {
		Status      string `json:"status"`
		FailedCount int    `json:"failed_count"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse metadata manifest: %v", err)
	}
	if manifest.Status != "failed" || manifest.FailedCount != 1 {
		t.Fatalf("manifest = %#v, want failed run", manifest)
	}
}

func metadataTestOptions(vaultDir string) Options {
	return Options{
		VaultPath:       vaultDir,
		RootFolder:      "zettelkasten",
		DataFolder:      ".aicli-zettel-merge",
		ProviderID:      "fake",
		MergeProviderID: "fake",
		MergeModel:      "metadata-model",
	}
}
