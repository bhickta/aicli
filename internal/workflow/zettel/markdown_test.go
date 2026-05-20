package zettel

import "testing"

func TestLineRangeExtractionAndRemoval(t *testing.T) {
	t.Parallel()

	content := "- keep one\n- remove one\n- remove two\n- keep two\n"
	ranges := []LineRange{
		{StartLine: 3, EndLine: 3, Reason: "same fact"},
		{StartLine: 2, EndLine: 2, Reason: "same fact"},
	}

	extracted := extractLineRanges(content, ranges)
	if extracted != "- remove one\n- remove two" {
		t.Fatalf("extracted = %q", extracted)
	}
	remaining := removeLineRanges(content, ranges)
	if remaining != "- keep one\n- keep two" {
		t.Fatalf("remaining = %q", remaining)
	}
}

func TestApplyMergePlanPreservesInsertionOrderForSameLine(t *testing.T) {
	t.Parallel()

	content := "first\nlast"
	got := applyMergePlan(content, MergePlan{Insertions: []Insertion{
		{AfterLine: 1, Markdown: "a"},
		{AfterLine: 1, Markdown: "b"},
	}})
	want := "first\na\nb\nlast"
	if got != want {
		t.Fatalf("merged = %q, want %q", got, want)
	}
}

func TestNormalizeRangesRejectsOutOfBoundsLine(t *testing.T) {
	t.Parallel()

	_, err := normalizeRanges([]LineRange{{StartLine: 2, EndLine: 4}}, "one\ntwo", 2)
	if err == nil {
		t.Fatal("expected out-of-bounds range error")
	}
}
