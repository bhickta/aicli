package notetext

import (
	"testing"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

func TestLineRangeExtractionAndRemoval(t *testing.T) {
	t.Parallel()

	content := "- keep one\n- remove one\n- remove two\n- keep two\n"
	ranges := []model.LineRange{
		{StartLine: 3, EndLine: 3, Reason: "same fact"},
		{StartLine: 2, EndLine: 2, Reason: "same fact"},
	}

	extracted := ExtractLineRanges(content, ranges)
	if extracted != "- remove one\n- remove two" {
		t.Fatalf("extracted = %q", extracted)
	}
	remaining := RemoveLineRanges(content, ranges)
	if remaining != "- keep one\n- keep two" {
		t.Fatalf("remaining = %q", remaining)
	}
}

func TestApplyMergePlanPreservesInsertionOrderForSameLine(t *testing.T) {
	t.Parallel()

	content := "first\nlast"
	got := ApplyMergePlan(content, model.MergePlan{Insertions: []model.Insertion{
		{AfterLine: 1, Markdown: "a"},
		{AfterLine: 1, Markdown: "b"},
	}})
	want := "first\na\nb\nlast"
	if got != want {
		t.Fatalf("merged = %q, want %q", got, want)
	}
}
