package zettel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestServiceInboxMergeProcessesSourceAndRollbackRestores(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md"), "- **Inflation**:: 6%\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "batch", "inflation.md"), "Inflation rose to 7% due to oil prices.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Inflation = 7% due to oil prices","source":"Inflation rose to 7% due to oil prices."}],"destinations":[{"path":"zettelkasten/economy.md","confidence":0.99,"final_markdown":"- **Inflation**:: 6%\n- **Inflation**:: 7% (Oil prices ^)","ledger":[{"claim_id":"c1","status":"merged","destination_path":"zettelkasten/economy.md","evidence":"added 7% oil price line","reason":"new fact"}],"reason":"inflation destination"}],"pending":[],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"ok"},"notes":"ok"}`,
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

func TestServiceInboxMergeProcessesExactDuplicateWithoutProviders(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	content := "---\nStatus: Read\n---\n- **Conceptual Clarity**: Economics = technical + conceptual.\n"
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "Economy", "Economy Shivin", "002.md")
	sourcePath := filepath.Join(vaultDir, "in", "Economy Shivin", "002.md")
	writeTestFile(t, destinationPath, content)
	writeTestFile(t, sourcePath, content)

	provider := &fakeZettelProvider{}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "in",
		CandidateProviderID:  "fake",
		MergeProviderID:      "fake",
		ValidationProviderID: "fake",
		EmbeddingProviderID:  "fake",
		CandidateModel:       "judge-model",
		MergeModel:           "merge-model",
		ValidationModel:      "validation-model",
		EmbeddingModel:       "embedding-model",
	}

	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v", err)
	}
	if resp.ProcessedCount != 1 || resp.PendingCount != 0 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want exact duplicate processed mechanically", resp)
	}
	if len(provider.chatCalls) != 0 || provider.embeddingCalls != 0 {
		t.Fatalf("provider calls chat=%d embedding=%d, want exact duplicate path to avoid providers", len(provider.chatCalls), provider.embeddingCalls)
	}
	processed := resp.Processed[0]
	if processed.DedupedCount != 1 || len(processed.DestinationPaths) != 1 {
		t.Fatalf("processed result = %#v, want one deduped destination", processed)
	}
	if processed.DestinationPaths[0] != "zettelkasten/Economy/Economy Shivin/002.md" {
		t.Fatalf("destination path = %q, want original matching note", processed.DestinationPaths[0])
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	if got := readTestFile(t, destinationPath); got != content {
		t.Fatalf("destination changed = %q, want exact original content", got)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if manifest.Status != "completed" || manifest.ProcessedCount != 1 || manifest.PendingCount != 0 || manifest.FailedCount != 0 {
		t.Fatalf("manifest = %#v, want completed exact duplicate run", manifest)
	}
}

func TestServiceInboxMergeAcceptsConceptLevelEconomicsMerge(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "Economics Definition.md")
	sourcePath := filepath.Join(vaultDir, "in", "Economy Shivin", "002_Conceptual_Clarity.md")
	writeTestFile(t, destinationPath, strings.Join([]string{
		"---",
		"Status: Read",
		"---",
		"- **Economics (Etymology)**: Oikos (Household) + Nomos (Management).",
		"- **Economics (Definition)**: Household management using limited resources to satisfy unlimited wants.",
		"",
	}, "\n"))
	writeTestFile(t, sourcePath, strings.Join([]string{
		"---",
		"Status: Read",
		"---",
		"- **Subject Nature**: Economics = technical + conceptual. Rote learning fails in UPSC.",
		"  - **Example**: 2022 Mains statement \"Economic growth led by labor productivity\" -> answer = Jobless Growth.",
		"- **Conceptual Clarity**: Understanding roots > memorizing definitions.",
		"  - **Roots**: Anthrop=man, Phil/Phile=love, Mis=hate, Ology=study, Ped=child, Gyne=woman.",
		"",
	}, "\n"))

	finalMarkdown := strings.Join([]string{
		"---",
		"Status: Read",
		"---",
		"- **Economics (Etymology)**: Oikos (Household) + Nomos (Management).",
		"- **Economics (Definition)**: Household management using limited resources to satisfy unlimited wants.",
		"  - **Preparation Lens**: Economics = technical + conceptual; rote learning fails, so roots/logic > memorized definitions.",
		"  - **Example**: 2022 Mains statement \"Economic growth led by labor productivity\" -> answer = Jobless Growth.",
		"  - **Root Examples**: Anthrop=man, Phil/Phile=love, Mis=hate, Ology=study, Ped=child, Gyne=woman.",
		"",
	}, "\n")
	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"concept-1","text":"Economics definition and conceptual clarity: technical+conceptual nature, rote learning fails, 2022 Jobless Growth example, and root-word method.","source":"whole source note"}],"destinations":[{"path":"zettelkasten/Economics Definition.md","confidence":0.99,"final_markdown":` + strconv.Quote(finalMarkdown) + `,"ledger":[{"claim_id":"concept-1","status":"merged","destination_path":"zettelkasten/Economics Definition.md","evidence":"Preparation Lens, Example, and Root Examples bullets","reason":"same overall Economics definition/conceptual clarity note"}],"reason":"same economics definition concept"}],"pending":[],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"concept represented"},"notes":"merged as one concept unit"}`,
	}}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "in",
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
		t.Fatalf("InboxMerge() = %#v, want concept-level processed note", resp)
	}
	processed := resp.Processed[0]
	if processed.MergedCount != 1 || len(processed.Claims) != 1 {
		t.Fatalf("processed result = %#v, want one merged concept unit", processed)
	}
	systemPrompt := provider.chatCalls[0].Messages[0].Content
	if !strings.Contains(systemPrompt, "Extract coherent concept units") || !strings.Contains(systemPrompt, "line-by-line atomic fragments") {
		t.Fatalf("system prompt does not pin concept-level extraction: %s", systemPrompt)
	}
	if got := readTestFile(t, destinationPath); !strings.Contains(got, "Economics = technical + conceptual") || !strings.Contains(got, "Jobless Growth") {
		t.Fatalf("destination content = %q, want economics concept details merged", got)
	}
}

func TestServiceInboxMergeDoesNotWriteDedupedOnlyDestination(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "polity.md")
	sourcePath := filepath.Join(vaultDir, "inbox-to-merge", "rote.md")
	destinationContent := "- **UPSC**: Rote learning fails; independent thinking needed.\n"
	writeTestFile(t, destinationPath, destinationContent)
	writeTestFile(t, sourcePath, "Rote learning fails in UPSC.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Rote learning fails in UPSC","source":"Rote learning fails in UPSC."}],"destinations":[{"path":"zettelkasten/polity.md","confidence":0.99,"final_markdown":"- **UPSC**: BROKEN STYLE REWRITE.\n","ledger":[{"claim_id":"c1","status":"deduped","destination_path":"zettelkasten/polity.md","evidence":"existing rote learning line","reason":"already represented"}],"reason":"already represented"}],"pending":[],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"dedupe verified"},"notes":"deduped"}`,
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
		t.Fatalf("InboxMerge() = %#v, want deduped note processed", resp)
	}
	processed := resp.Processed[0]
	if processed.MergedCount != 0 || processed.DedupedCount != 1 || len(processed.Diffs) != 0 {
		t.Fatalf("processed result = %#v, want dedupe-only no-write result", processed)
	}
	if got := readTestFile(t, destinationPath); got != destinationContent {
		t.Fatalf("deduped destination changed = %q, want %q", got, destinationContent)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	if processed.ProcessedPath == "" || !strings.HasPrefix(processed.ProcessedPath, "inbox-to-merge/_processed/") {
		t.Fatalf("processed path = %q, want processed folder path", processed.ProcessedPath)
	}
	if resp.APICalls.Total != 2 || resp.APICalls.Chat != 1 || resp.APICalls.Embeddings != 1 {
		t.Fatalf("api calls = %#v, want one chat call and one embedding call", resp.APICalls)
	}
}

func TestServiceInboxMergeUsesRouteLevelDedupeWithoutRewrite(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "polity.md")
	sourcePath := filepath.Join(vaultDir, "inbox-to-merge", "rote.md")
	destinationContent := "- **UPSC**: Rote learning fails; independent thinking needed.\n"
	writeTestFile(t, destinationPath, destinationContent)
	writeTestFile(t, sourcePath, "Rote learning fails in UPSC.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Rote learning fails in UPSC","source":"Rote learning fails in UPSC."}],"destinations":[{"path":"zettelkasten/polity.md","confidence":0.99,"ledger":[{"claim_id":"c1","status":"deduped","destination_path":"zettelkasten/polity.md","evidence":"excerpt already says rote learning fails","reason":"already represented"}],"reason":"already represented"}],"pending":[],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"route dedupe verified"},"notes":"ok"}`,
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
		t.Fatalf("InboxMerge() = %#v, want route-deduped note processed", resp)
	}
	if got := readTestFile(t, destinationPath); got != destinationContent {
		t.Fatalf("deduped destination changed = %q, want %q", got, destinationContent)
	}
	if len(provider.chatCalls) != 1 {
		t.Fatalf("chat calls = %d, want one decision call", len(provider.chatCalls))
	}
	if provider.chatCalls[0].Model != "merge-model" {
		t.Fatalf("chat model = %q, want merge-model for one-shot inbox decision", provider.chatCalls[0].Model)
	}
	if resp.APICalls.Total != 2 || resp.APICalls.Chat != 1 || resp.APICalls.Embeddings != 1 {
		t.Fatalf("api calls = %#v, want one chat call and one embedding call", resp.APICalls)
	}
}

func TestServiceInboxMergeAddsLexicalCandidatesBeyondEmbeddingLimit(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	joblessPath := filepath.Join(vaultDir, "zettelkasten", "zzz_jobless.md")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "aaa_unrelated.md"), "- **Inflation**:: WPI basket changed.\n")
	writeTestFile(t, joblessPath, "- **Jobless Growth**: Services = high value/low labor.\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "mixed.md"), "- **Example**: 2022 Mains answer = **Jobless Growth**.\n- **Root**: Anthrop = man.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"2022 Mains answer is Jobless Growth","source":"Example"},{"id":"c2","text":"Anthrop means man","source":"Root"}],"destinations":[{"path":"zettelkasten/zzz_jobless.md","confidence":0.99,"final_markdown":"- **Jobless Growth**: Services = high value/low labor.\n- **2022 Mains**: answer = Jobless Growth.","ledger":[{"claim_id":"c1","status":"merged","destination_path":"zettelkasten/zzz_jobless.md","evidence":"added 2022 mains answer","reason":"same concept"}],"reason":"lexical candidate matched Jobless Growth"}],"pending":[{"claim_id":"c2","status":"pending","reason":"no destination for root word"}],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"ok"},"notes":"partial"}`,
	}}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "inbox-to-merge",
		CandidateLimit:       1,
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
		t.Fatalf("InboxMerge() = %#v, want partial result with one merged lexical candidate", resp)
	}
	if len(provider.chatCalls) != 1 {
		t.Fatalf("chat calls = %d, want one decision call", len(provider.chatCalls))
	}
	decisionPayload := provider.chatCalls[0].Messages[len(provider.chatCalls[0].Messages)-1].Content
	if !strings.Contains(decisionPayload, "zettelkasten/zzz_jobless.md") {
		t.Fatalf("decision payload missing lexical jobless candidate: %s", decisionPayload)
	}
	if got := readTestFile(t, joblessPath); !strings.Contains(got, "2022 Mains") {
		t.Fatalf("jobless destination = %q, want merged mains example", got)
	}
	if resp.APICalls.Total != 2 || resp.APICalls.Chat != 1 || resp.APICalls.Embeddings != 1 {
		t.Fatalf("api calls = %#v, want one chat call and one embedding call", resp.APICalls)
	}
}

func TestServiceInboxMergeAdoptsUnmatchedSourceWhenEnabled(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "Economy", "existing.md"), "- **Economy**: existing index seed.\n")
	sourcePath := filepath.Join(vaultDir, "in", "Economy Shivin", "002.md")
	sourceContent := "- **Conceptual Clarity**: Understanding roots > memorizing definitions.\n"
	writeTestFile(t, sourcePath, sourceContent)

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Understanding roots is better than memorizing definitions","source":"Conceptual Clarity"}],"destinations":[],"pending":[{"claim_id":"c1","status":"pending","reason":"no confident destination"}],"notes":"pending"}`,
	}}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
		VaultPath:            vaultDir,
		RootFolder:           "zettelkasten",
		DataFolder:           ".aicli-zettel-merge",
		InboxFolder:          "in",
		AdoptUnmatchedInbox:  true,
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
		t.Fatalf("InboxMerge() = %#v, want adopted source processed", resp)
	}
	adoptedPath := filepath.Join(vaultDir, "zettelkasten", "Economy", "Economy Shivin", "002.md")
	if got := readTestFile(t, adoptedPath); got != sourceContent {
		t.Fatalf("adopted destination = %q, want source content", got)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	processed := resp.Processed[0]
	if processed.MergedCount != 1 || processed.PendingCount != 0 || len(processed.Diffs) != 1 || !processed.Diffs[0].Created {
		t.Fatalf("processed result = %#v, want one created destination merge", processed)
	}
	if len(provider.chatCalls) != 1 {
		t.Fatalf("chat calls = %d, want one decision call", len(provider.chatCalls))
	}
	if provider.chatCalls[0].Model != "merge-model" {
		t.Fatalf("chat model = %q, want merge-model for one-shot inbox decision", provider.chatCalls[0].Model)
	}
	if resp.APICalls.Total != 2 || resp.APICalls.Chat != 1 || resp.APICalls.Embeddings != 1 {
		t.Fatalf("api calls = %#v, want one chat call and one embedding call", resp.APICalls)
	}

	if _, err := service.Rollback(context.Background(), RollbackRequest{Options: options, JobID: resp.RunID}, nil); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := os.Stat(adoptedPath); !os.IsNotExist(err) {
		t.Fatalf("rollback left adopted destination or unexpected stat error: %v", err)
	}
	if got := readTestFile(t, sourcePath); got != sourceContent {
		t.Fatalf("rollback source = %q, want original content", got)
	}
}

func TestServiceInboxMergeKeepsPendingSourceUnchanged(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "economy.md"), "- **Inflation**:: 6%\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "stray.md"), "Ambiguous note with no safe destination.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Ambiguous note with no safe destination","source":"Ambiguous note with no safe destination."}],"destinations":[],"pending":[{"claim_id":"c1","status":"pending","reason":"no confident destination"}],"notes":"pending"}`,
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

func TestServiceInboxMergeAppliesPartialAndPreservesPendingSource(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "economy.md")
	sourcePath := filepath.Join(vaultDir, "inbox-to-merge", "mixed.md")
	writeTestFile(t, destinationPath, "- **Inflation**:: 6%\n")
	writeTestFile(t, sourcePath, "Inflation rose to 7%.\nUnclear prelims range.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Inflation rose to 7%","source":"Inflation rose to 7%."},{"id":"c2","text":"Unclear prelims range","source":"Unclear prelims range."}],"destinations":[{"path":"zettelkasten/economy.md","confidence":0.99,"final_markdown":"- **Inflation**:: 6%\n- **Inflation**:: 7%","ledger":[{"claim_id":"c1","status":"merged","destination_path":"zettelkasten/economy.md","evidence":"added inflation 7%","reason":"new fact"}],"reason":"inflation destination"}],"pending":[{"claim_id":"c2","status":"pending","reason":"no confident destination"}],"validation":{"verdict":"pass","score":1,"missing_facts":[],"unsupported_additions":[],"notes":"partial applied facts are represented"},"notes":"partial"}`,
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
	if resp.ProcessedCount != 0 || resp.PendingCount != 1 || resp.FailedCount != 0 {
		t.Fatalf("InboxMerge() = %#v, want one partial pending note", resp)
	}
	if resp.APICalls.Total != 2 || resp.APICalls.Chat != 1 || resp.APICalls.Embeddings != 1 {
		t.Fatalf("api calls = %#v, want one chat call and one embedding call", resp.APICalls)
	}
	partial := resp.Pending[0]
	if partial.Status != "partial" || partial.ProcessedPath == "" || !strings.HasPrefix(partial.ProcessedPath, "inbox-to-merge/_pending/") {
		t.Fatalf("partial result = %#v, want pending folder path", partial)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or unexpected stat error: %v", err)
	}
	if got := readTestFile(t, destinationPath); got != "- **Inflation**:: 6%\n- **Inflation**:: 7%\n" {
		t.Fatalf("destination content = %q, want partial applied merge", got)
	}
	manifest := readInboxRunManifest(t, resp.ArchivePath)
	if manifest.Status != "partial" || manifest.PendingCount != 1 {
		t.Fatalf("manifest = %#v, want partial run", manifest)
	}

	if _, err := service.Rollback(context.Background(), RollbackRequest{Options: options, JobID: resp.RunID}, nil); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got := readTestFile(t, destinationPath); got != "- **Inflation**:: 6%\n" {
		t.Fatalf("rollback destination = %q", got)
	}
	if got := readTestFile(t, sourcePath); got != "Inflation rose to 7%.\nUnclear prelims range.\n" {
		t.Fatalf("rollback source = %q", got)
	}
}

func TestInboxRollbackIgnoresPendingDestinationArchives(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	destinationPath := filepath.Join(vaultDir, "zettelkasten", "economy.md")
	sourcePath := filepath.Join(vaultDir, "inbox-to-merge", "stray.md")
	writeTestFile(t, destinationPath, "- **Inflation**:: 6%\n")
	writeTestFile(t, sourcePath, "Inflation rose to 7%, but routing is uncertain.\n")

	provider := &fakeZettelProvider{chatResponses: []string{
		`{"claims":[{"id":"c1","text":"Inflation rose to 7%","source":"Inflation rose to 7%"}],"destinations":[{"path":"zettelkasten/economy.md","confidence":0.99,"ledger":[{"claim_id":"c1","status":"pending","destination_path":"zettelkasten/economy.md","reason":"not enough evidence to merge"}],"reason":"inflation destination"}],"pending":[],"notes":"pending"}`,
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
		t.Fatalf("InboxMerge() = %#v, want pending run", resp)
	}

	writeTestFile(t, destinationPath, "- **Inflation**:: manual edit after pending run\n")
	if _, err := service.Rollback(context.Background(), RollbackRequest{Options: options, JobID: resp.RunID}, nil); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got := readTestFile(t, destinationPath); got != "- **Inflation**:: manual edit after pending run\n" {
		t.Fatalf("rollback changed unapplied pending destination = %q", got)
	}
	if got := readTestFile(t, sourcePath); got != "Inflation rose to 7%, but routing is uncertain.\n" {
		t.Fatalf("pending source changed = %q", got)
	}
}

func TestServiceInboxMergeRespectsInboxLimit(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "destination.md"), "destination\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "001.md"), "first\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "002.md"), "second\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "003.md"), "third\n")

	provider := &fakeZettelProvider{}
	service := NewWithProviders(provider, provider, provider, provider)
	options := Options{
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
	}
	if _, err := service.Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	resp, err := service.InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
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

func TestServiceInboxMergeFailsFastWhenEmbeddingProviderUnavailable(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "destination.md"), "destination\n")
	writeTestFile(t, filepath.Join(vaultDir, "inbox-to-merge", "001.md"), "first\n")

	indexProvider := &fakeZettelProvider{}
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
	if _, err := NewWithProviders(indexProvider, indexProvider, indexProvider, indexProvider).Index(context.Background(), IndexRequest{Options: options}, nil); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	embeddingProvider := &fakeZettelProvider{embeddingErr: errors.New("dial tcp 127.0.0.1:1234: connect: connection refused")}
	chatProvider := &fakeZettelProvider{}
	resp, err := NewWithProviders(chatProvider, chatProvider, chatProvider, embeddingProvider).InboxMerge(context.Background(), InboxMergeRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("InboxMerge() error = %v, want per-note failure response", err)
	}
	if len(chatProvider.chatCalls) != 0 {
		t.Fatalf("chat calls = %d, want fail before routing", len(chatProvider.chatCalls))
	}
	if resp.SelectedCount != 1 || resp.FailedCount != 1 || resp.ProcessedCount != 0 {
		t.Fatalf("InboxMerge() response = %#v, want selected count with failed note", resp)
	}
	if !strings.Contains(resp.Failed[0].Reason, "connect: connection refused") {
		t.Fatalf("failed reason = %q, want embedding provider error", resp.Failed[0].Reason)
	}
}

type testInboxRunManifest struct {
	Status         string `json:"status"`
	ProcessedCount int    `json:"processed_count"`
	PendingCount   int    `json:"pending_count"`
	FailedCount    int    `json:"failed_count"`
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
