package zettel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceApplyClipsExactRangesAndRollbackRestores(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"), "# IMF\n\n- existing fact\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "source.md"), "- copied one\n- copied two\n- keep source\n")

	sourceContent := "- copied one\n- copied two\n- keep source\n"
	proposal := Proposal{
		ID:            "job-test",
		CreatedAt:     time.Now().UTC(),
		VaultPath:     vaultDir,
		RootFolder:    "zettelkasten",
		DataFolder:    ".aicli-zettel-merge",
		ActivePath:    "zettelkasten/active.md",
		ActiveHash:    hashText("# IMF\n\n- existing fact\n"),
		FinalMarkdown: "# IMF\n\n- existing fact\n- copied one\n- copied two\n",
		SourceExtractions: []SourceExtraction{{
			Path:              "zettelkasten/source.md",
			OriginalHash:      hashText(sourceContent),
			SourceLineRanges:  []LineRange{{StartLine: 1, EndLine: 2}},
			ExtractedMarkdown: "- copied one\n- copied two",
		}},
		Coverage: CoverageReport{Score: 1},
		Judge:    MergeJudge{Verdict: "pass", Score: 1},
	}

	service := New(nil)
	apply, err := service.Apply(context.Background(), ApplyRequest{Options: proposalOptions(proposal), Proposal: proposal}, nil)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if apply.JobID != proposal.ID {
		t.Fatalf("job id = %q, want %q", apply.JobID, proposal.ID)
	}

	active := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"))
	if active != proposal.FinalMarkdown {
		t.Fatalf("active content = %q, want final markdown", active)
	}
	source := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "source.md"))
	if source != "- keep source\n" {
		t.Fatalf("source content = %q, want only unselected source line", source)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".aicli-zettel-merge", "jobs", proposal.ID, "manifest.json")); err != nil {
		t.Fatalf("archive manifest missing: %v", err)
	}

	rollback, err := service.Rollback(context.Background(), RollbackRequest{Options: proposalOptions(proposal), JobID: proposal.ID}, nil)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rollback.JobID != proposal.ID {
		t.Fatalf("rollback job id = %q, want %q", rollback.JobID, proposal.ID)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md")); got != "# IMF\n\n- existing fact\n" {
		t.Fatalf("restored active = %q", got)
	}
	if got := readTestFile(t, filepath.Join(vaultDir, "zettelkasten", "source.md")); got != sourceContent {
		t.Fatalf("restored source = %q", got)
	}
}

func TestServiceApplyRejectsChangedSource(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"), "active\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "source.md"), "changed\n")

	proposal := Proposal{
		ID:            "job-source-changed",
		CreatedAt:     time.Now().UTC(),
		VaultPath:     vaultDir,
		RootFolder:    "zettelkasten",
		DataFolder:    ".aicli-zettel-merge",
		ActivePath:    "zettelkasten/active.md",
		ActiveHash:    hashText("active\n"),
		FinalMarkdown: "active\nmerged\n",
		SourceExtractions: []SourceExtraction{{
			Path:              "zettelkasten/source.md",
			OriginalHash:      hashText("original\n"),
			SourceLineRanges:  []LineRange{{StartLine: 1, EndLine: 1}},
			ExtractedMarkdown: "original",
		}},
	}

	_, err := New(nil).Apply(context.Background(), ApplyRequest{Options: proposalOptions(proposal), Proposal: proposal}, nil)
	if err == nil {
		t.Fatal("expected source hash guard error")
	}
}

func proposalOptions(proposal Proposal) Options {
	return Options{
		VaultPath:  proposal.VaultPath,
		RootFolder: proposal.RootFolder,
		DataFolder: proposal.DataFolder,
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
