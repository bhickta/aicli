package zettel

import "github.com/bhickta/aicli/internal/workflow/zettel/notetext"

func splitLines(content string) []string {
	return notetext.SplitLines(content)
}

func ensureTrailingNewline(content string) string {
	return notetext.EnsureTrailingNewline(content)
}

func hashText(content string) string {
	return notetext.HashText(content)
}

func numberedNote(path string, content string) string {
	return notetext.NumberedNote(path, content)
}

func numberedExcerpt(path string, content string, maxChars int) (string, int) {
	return notetext.NumberedExcerpt(path, content, maxChars)
}

func extractLineRanges(content string, ranges []LineRange) string {
	return notetext.ExtractLineRanges(content, ranges)
}

func removeLineRanges(content string, ranges []LineRange) string {
	return notetext.RemoveLineRanges(content, ranges)
}

func mergeLineRanges(ranges []LineRange) []LineRange {
	return notetext.MergeLineRanges(ranges)
}

func applyMergePlan(target string, plan MergePlan) string {
	return notetext.ApplyMergePlan(target, plan)
}

func compactNote(path string, content string, maxChars int) string {
	return notetext.CompactNote(path, content, maxChars)
}
