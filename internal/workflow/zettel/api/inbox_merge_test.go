package zettel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceInboxMergeWritesFinalNoteAndRollbackRestores(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "economy.md")
	sourcePath := filepath.Join(vaultDir, "inbox-to-merge", "batch", "inflation.md")
	writeTestFile(t, destinationPath, "- **Inflation**: 6%.\n")
	writeTestFile(t, sourcePath, "Inflation rose to 7% due to oil prices.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		strings.Join([]string{
			"BEGIN_NOTE zettelkasten/economy.md",
			"- **Inflation**: 6%.",
			"- **Inflation Spike**: 7% due to oil prices.",
			"END_NOTE",
			"",
		}, "\n"),
	}}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "inbox-to-merge")
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
	if len(provider.chatCalls) != 1 {
		t.Fatalf("chat calls = %d, want one AI merge call", len(provider.chatCalls))
	}
	systemPrompt := provider.chatCalls[0].Messages[0].Content
	if !strings.Contains(systemPrompt, "Return final destination notes only") || !strings.Contains(systemPrompt, "Do not return JSON") {
		t.Fatalf("system prompt does not require final-note response: %s", systemPrompt)
	}
	if got := readTestFile(t, destinationPath); !strings.Contains(got, "Inflation Spike") {
		t.Fatalf("destination content = %q, want merged final note", got)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resp.ArchivePath, "manifest.json")); err != nil {
		t.Fatalf("inbox manifest missing: %v", err)
	}
	llmArchives, err := filepath.Glob(filepath.Join(resp.ArchivePath, "llm", "*.json"))
	if err != nil {
		t.Fatalf("glob llm archives: %v", err)
	}
	if len(llmArchives) != 1 {
		t.Fatalf("llm archives = %v, want saved merge request/response", llmArchives)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if len(manifest.Items) != 1 || manifest.Items[0].AuditArchive == "" {
		t.Fatalf("manifest items = %#v, want one item with audit archive", manifest.Items)
	}
	audit := readInboxAuditItem(t, resp.ArchivePath, manifest.Items[0].AuditArchive)
	if audit.SourcePath != "inbox-to-merge/batch/inflation.md" || audit.SourceContent != "Inflation rose to 7% due to oil prices.\n" {
		t.Fatalf("audit source = %#v, want archived source path and content", audit)
	}
	if audit.ProcessedPath == "" {
		t.Fatalf("audit processed path is empty, want moved source path")
	}
	if len(audit.Destinations) != 1 {
		t.Fatalf("audit destinations = %#v, want one destination", audit.Destinations)
	}
	destination := audit.Destinations[0]
	if destination.Path != "zettelkasten/economy.md" {
		t.Fatalf("audit destination path = %q, want zettelkasten/economy.md", destination.Path)
	}
	if !strings.Contains(destination.Before, "Inflation") || !strings.Contains(destination.After, "Inflation Spike") || destination.Diff == "" {
		t.Fatalf("audit destination = %#v, want before/after/diff content", destination)
	}

	rollback, err := service.Rollback(context.Background(), RollbackRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rollback.JobID != resp.RunID {
		t.Fatalf("rollback job id = %q, want %q", rollback.JobID, resp.RunID)
	}
	if got := readTestFile(t, destinationPath); got != "- **Inflation**: 6%.\n" {
		t.Fatalf("restored destination = %q", got)
	}
	if got := readTestFile(t, sourcePath); got != "Inflation rose to 7% due to oil prices.\n" {
		t.Fatalf("restored source = %q", got)
	}
}

func TestServiceInboxMergeProcessesExactDuplicateWithoutProviders(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	content := "---\nStatus: Read\n---\n- **Conceptual Clarity**: Economics = technical + conceptual.\n"
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "Economy", "Economy Shivin", "002.md")
	sourcePath := filepath.Join(vaultDir, "in", "Economy Shivin", "002.md")
	writeTestFile(t, destinationPath, content)
	writeTestFile(t, sourcePath, content)

	provider := &fakeZettelProvider{}
	options := inboxMergeTestOptions(vaultDir, "in")
	resp, err := NewWithProviders(provider, provider).InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 1 || resp.PendingCount != 0 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want exact duplicate processed mechanically", resp)
	}
	if len(provider.chatCalls) != 0 || provider.embeddingCalls != 0 {
		t.Fatalf("provider calls chat=%d embedding=%d, want exact duplicate path to avoid providers", len(provider.chatCalls), provider.embeddingCalls)
	}
	if got := readTestFile(t, destinationPath); got != content {
		t.Fatalf("destination changed = %q, want exact original content", got)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if manifest.Status != "completed" || manifest.ProcessedCount != 1 || manifest.PendingCount != 0 || manifest.FailedCount != 0 {
		t.Fatalf("manifest = %#v, want completed exact duplicate run", manifest)
	}
	if len(manifest.Items) != 1 || manifest.Items[0].AuditArchive == "" {
		t.Fatalf("manifest items = %#v, want exact duplicate audit archive", manifest.Items)
	}
	audit := readInboxAuditItem(t, resp.ArchivePath, manifest.Items[0].AuditArchive)
	if len(audit.Destinations) != 1 {
		t.Fatalf("audit destinations = %#v, want duplicate destination", audit.Destinations)
	}
	destination := audit.Destinations[0]
	if destination.Path != "zettelkasten/Economy/Economy Shivin/002.md" {
		t.Fatalf("audit destination path = %q, want exact duplicate destination", destination.Path)
	}
	if destination.Before != content || destination.After != content || destination.Diff != "" {
		t.Fatalf("audit destination = %#v, want identical before/after without diff", destination)
	}
}

func TestServiceInboxMergeWritesMultipleAtomicFinalNotes(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	microPath := filepath.Join(vaultDir, "zettelkasten", "microeconomics.md")
	macroPath := filepath.Join(vaultDir, "zettelkasten", "macroeconomics.md")
	sourcePath := filepath.Join(vaultDir, "in", "economics.md")
	writeTestFile(t, microPath, "- **Microeconomics**: Study of a firm or household.\n")
	writeTestFile(t, macroPath, "- **Macroeconomics**: Whole economy and national income.\n")
	writeTestFile(t, sourcePath, "- **Microeconomics**: Microscope view.\n- **Macroeconomics**: Telescope view.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		strings.Join([]string{
			"BEGIN_NOTE zettelkasten/microeconomics.md",
			"- **Microeconomics**: Study of a firm or household.",
			"- **Microeconomics**: Microscope view.",
			"END_NOTE",
			"BEGIN_NOTE zettelkasten/macroeconomics.md",
			"- **Macroeconomics**: Whole economy and national income.",
			"- **Macroeconomics**: Telescope view.",
			"END_NOTE",
			"",
		}, "\n"),
	}}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 1 || resp.PendingCount != 0 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want processed multi-destination merge", resp)
	}
	if !strings.Contains(readTestFile(t, microPath), "Microscope") || !strings.Contains(readTestFile(t, macroPath), "Telescope") {
		t.Fatalf("destinations did not receive source facts")
	}
}

func TestServiceInboxMergeDoesNotAdoptNewFinalNote(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	sourcePath := filepath.Join(vaultDir, "in", "Economy Shivin", "003.md")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Economy**: seed.\n")
	writeTestFile(t, sourcePath, "- **Prelims Syllabus**: Poverty, inclusion, and demographics.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		strings.Join([]string{
			"BEGIN_NOTE zettelkasten/Economy Shivin/003.md",
			"- **Prelims Syllabus**: Poverty, inclusion, and demographics.",
			"END_NOTE",
			"",
		}, "\n"),
	}}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 0 || resp.PendingCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want non-candidate new note kept pending", resp)
	}
	newPath := filepath.Join(vaultDir, "zettelkasten", "Economy Shivin", "003.md")
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Fatalf("new destination was created or stat error: %v", err)
	}
}

func TestServiceInboxMergeKeepsSourcePendingWhenModelDeclines(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	sourcePath := filepath.Join(vaultDir, "in", "misc.md")
	sourceContent := "- **Unmatched**: unrelated inbox fact.\n"
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Economy**: seed.\n")
	writeTestFile(t, sourcePath, sourceContent)

	provider := &fakeZettelProvider{chatResponse: "PENDING: no semantically similar destination note\n"}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 0 || resp.PendingCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want pending source", resp)
	}
	if got := readTestFile(t, sourcePath); got != sourceContent {
		t.Fatalf("source changed = %q, want unchanged pending source", got)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if len(manifest.Items) != 1 || manifest.Items[0].AuditArchive == "" {
		t.Fatalf("manifest items = %#v, want pending audit archive", manifest.Items)
	}
	audit := readInboxAuditItem(t, resp.ArchivePath, manifest.Items[0].AuditArchive)
	if audit.Status != "pending" || audit.SourceContent != sourceContent || !strings.Contains(audit.Reason, "no semantically similar") {
		t.Fatalf("audit = %#v, want pending source audit with reason", audit)
	}
	if len(audit.Destinations) != 0 {
		t.Fatalf("audit destinations = %#v, want none for pending source", audit.Destinations)
	}
}

func TestServiceInboxMergeRejectsNonCandidateFinalNote(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "economy.md")
	sourcePath := filepath.Join(vaultDir, "in", "inflation.md")
	writeTestFile(t, destinationPath, "- **Inflation**: 6%.\n")
	writeTestFile(t, sourcePath, "Inflation rose to 7% due to oil prices.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		strings.Join([]string{
			"BEGIN_NOTE zettelkasten/not-a-candidate.md",
			"- **Inflation Spike**: 7% due to oil prices.",
			"END_NOTE",
			"",
		}, "\n"),
	}}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 0 || resp.PendingCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want pending non-candidate destination", resp)
	}
	if got := readTestFile(t, destinationPath); got != "- **Inflation**: 6%.\n" {
		t.Fatalf("destination changed = %q, want unchanged", got)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "zettelkasten", "not-a-candidate.md")); !os.IsNotExist(err) {
		t.Fatalf("non-candidate note was created or stat error: %v", err)
	}
}

func TestServiceInboxMergeRespectsInboxLimit(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Seed**: candidate.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "a.md"), "- **A**: one.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "b.md"), "- **B**: two.\n")

	provider := &fakeZettelProvider{chatResponse: "PENDING: test run\n"}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	options.InboxLimit = 1
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.SourceCount != 2 || resp.SelectedCount != 1 || resp.SkippedCount != 1 || resp.Limit != 1 {
		t.Fatalf("InboxMerge() counts = %#v, want one selected and one skipped", resp)
	}
	if len(provider.chatCalls) != 1 {
		t.Fatalf("chat calls = %d, want one selected note", len(provider.chatCalls))
	}
}

func TestServiceInboxMergeRunsSelectedNotesWithParallelWorkers(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Seed**: candidate.\n")
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		writeTestFile(t, filepath.Join(vaultDir, "in", name), "- **Source**: "+name+".\n")
	}

	provider := &fakeZettelProvider{chatResponse: "PENDING: test run\n"}
	service := NewWithProviders(provider, provider)
	options := inboxMergeTestOptions(vaultDir, "in")
	options.InboxWorkers = 3
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.SelectedCount != 3 || resp.PendingCount != 3 || resp.ProcessedCount != 0 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want three pending notes processed by worker pool", resp)
	}
	if len(provider.chatCalls) != 3 {
		t.Fatalf("chat calls = %d, want one call per selected note", len(provider.chatCalls))
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if len(manifest.Items) != 3 {
		t.Fatalf("manifest items = %d, want all parallel results archived", len(manifest.Items))
	}
}

func TestServiceInboxMergeFailsFastWhenEmbeddingProviderUnavailable(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "seed.md"), "- **Seed**: candidate.\n")
	writeTestFile(t, filepath.Join(vaultDir, "in", "source.md"), "- **Source**: content.\n")

	options := inboxMergeTestOptions(vaultDir, "in")
	if _, err := NewWithEmbedding(nil, &fakeZettelProvider{}).Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	embeddingProvider := &fakeZettelProvider{embeddingErr: errors.New("dial tcp 127.0.0.1:1234: connect: connection refused")}
	chatProvider := &fakeZettelProvider{}
	resp, err := NewWithProviders(chatProvider, embeddingProvider).InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v, want per-note failure response", err)
	}
	if len(chatProvider.chatCalls) != 0 {
		t.Fatalf("chat calls = %d, want fail before merge model", len(chatProvider.chatCalls))
	}
	if resp.SelectedCount != 1 || resp.FailedCount != 1 || resp.ProcessedCount != 0 {
		t.Fatalf("InboxMerge() response = %#v, want selected count with failed note", resp)
	}
	if !strings.Contains(resp.Failed[0].Reason, "connect: connection refused") {
		t.Fatalf("failed reason = %q, want embedding provider error", resp.Failed[0].Reason)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if len(manifest.Items) != 1 || manifest.Items[0].AuditArchive == "" {
		t.Fatalf("manifest items = %#v, want failed audit archive", manifest.Items)
	}
	audit := readInboxAuditItem(t, resp.ArchivePath, manifest.Items[0].AuditArchive)
	if audit.Status != "failed" || !strings.Contains(audit.Reason, "connect: connection refused") {
		t.Fatalf("audit = %#v, want failed audit with provider error", audit)
	}
}

func inboxMergeTestOptions(vaultDir string, inboxFolder string) Options {
	return Options{
		VaultPath:           vaultDir,
		RootFolder:          "zettelkasten",
		DataFolder:          ".aicli-zettel-merge",
		InboxFolder:         inboxFolder,
		MergeProviderID:     "fake",
		EmbeddingProviderID: "fake",
		MergeModel:          "merge-model",
		EmbeddingModel:      "embedding-model",
	}
}

type testInboxRunManifest struct {
	Status         string       `json:"status"`
	ProcessedCount int          `json:"processed_count"`
	PendingCount   int          `json:"pending_count"`
	FailedCount    int          `json:"failed_count"`
	APICalls       APICallUsage `json:"api_calls"`
	Items          []struct {
		SourcePath    string `json:"source_path"`
		Status        string `json:"status"`
		Reason        string `json:"reason"`
		SourceArchive string `json:"source_archive"`
		AuditArchive  string `json:"audit_archive"`
	} `json:"items"`
}

type testInboxAuditItem struct {
	SourcePath    string                      `json:"source_path"`
	SourceArchive string                      `json:"source_archive"`
	SourceContent string                      `json:"source_content"`
	Status        string                      `json:"status"`
	ProcessedPath string                      `json:"processed_path"`
	Reason        string                      `json:"reason"`
	Destinations  []testInboxAuditDestination `json:"destinations"`
}

type testInboxAuditDestination struct {
	Path          string `json:"path"`
	Before        string `json:"before"`
	After         string `json:"after"`
	Diff          string `json:"diff"`
	BeforeArchive string `json:"before_archive"`
	AfterArchive  string `json:"after_archive"`
	DiffArchive   string `json:"diff_archive"`
}

func readInboxRunManifest(t *testing.T, archivePath string) testInboxRunManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(archivePath, "manifest.json"))
	if err != nil {
		t.Fatalf("read inbox manifest: %v", err)
	}
	var manifest testInboxRunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse inbox manifest: %v", err)
	}
	return manifest
}

func readInboxAuditItem(t *testing.T, archivePath string, auditArchive string) testInboxAuditItem {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(archivePath, auditArchive))
	if err != nil {
		t.Fatalf("read inbox audit item: %v", err)
	}
	var audit testInboxAuditItem
	if err := json.Unmarshal(data, &audit); err != nil {
		t.Fatalf("parse inbox audit item: %v", err)
	}
	return audit
}
