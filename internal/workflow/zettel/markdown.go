package zettel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func splitLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

func ensureTrailingNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func hashText(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func numberedNote(path string, content string) string {
	lines := splitLines(content)
	out := make([]string, 0, len(lines)+2)
	out = append(out, "PATH: "+path, "LINES:")
	for i, line := range lines {
		out = append(out, fmt.Sprintf("%4d | %s", i+1, line))
	}
	return strings.Join(out, "\n")
}

func numberedExcerpt(path string, content string, maxChars int) (string, int) {
	numbered := numberedNote(path, content)
	lines := strings.Split(numbered, "\n")
	if len(numbered) <= maxChars {
		return numbered, len(splitLines(content))
	}
	var out []string
	used := 0
	lastLine := 0
	for _, line := range lines {
		next := used + len(line)
		if len(out) > 0 {
			next++
		}
		if next > maxChars {
			break
		}
		out = append(out, line)
		used = next
		var n int
		if _, err := fmt.Sscanf(line, "%d |", &n); err == nil {
			lastLine = n
		}
	}
	out = append(out, fmt.Sprintf("[truncated after line %d]", lastLine))
	return strings.Join(out, "\n"), lastLine
}

func extractLineRanges(content string, ranges []LineRange) string {
	lines := splitLines(content)
	var parts []string
	for _, r := range mergeLineRanges(ranges) {
		if r.StartLine < 1 || r.EndLine > len(lines) || r.StartLine > r.EndLine {
			continue
		}
		parts = append(parts, strings.Join(lines[r.StartLine-1:r.EndLine], "\n"))
	}
	return strings.Join(parts, "\n")
}

func removeLineRanges(content string, ranges []LineRange) string {
	lines := splitLines(content)
	remove := make(map[int]bool)
	for _, r := range mergeLineRanges(ranges) {
		if r.StartLine < 1 || r.EndLine > len(lines) || r.StartLine > r.EndLine {
			continue
		}
		for line := r.StartLine; line <= r.EndLine; line++ {
			remove[line] = true
		}
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if !remove[i+1] {
			out = append(out, line)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func mergeLineRanges(ranges []LineRange) []LineRange {
	filtered := make([]LineRange, 0, len(ranges))
	for _, r := range ranges {
		if r.StartLine > 0 && r.EndLine >= r.StartLine {
			filtered = append(filtered, r)
		}
	}
	for i := 1; i < len(filtered); i++ {
		key := filtered[i]
		j := i - 1
		for j >= 0 && (filtered[j].StartLine > key.StartLine || filtered[j].StartLine == key.StartLine && filtered[j].EndLine > key.EndLine) {
			filtered[j+1] = filtered[j]
			j--
		}
		filtered[j+1] = key
	}
	merged := make([]LineRange, 0, len(filtered))
	for _, r := range filtered {
		if len(merged) == 0 || r.StartLine > merged[len(merged)-1].EndLine+1 {
			merged = append(merged, r)
			continue
		}
		last := &merged[len(merged)-1]
		if r.EndLine > last.EndLine {
			last.EndLine = r.EndLine
		}
		if r.Reason != "" && !strings.Contains(last.Reason, r.Reason) {
			if last.Reason != "" {
				last.Reason += "; "
			}
			last.Reason += r.Reason
		}
	}
	return merged
}

func applyMergePlan(target string, plan MergePlan) string {
	lines := splitLines(target)
	type indexedInsertion struct {
		insertion Insertion
		index     int
	}
	insertions := make([]indexedInsertion, 0, len(plan.Insertions))
	for i, insertion := range plan.Insertions {
		insertions = append(insertions, indexedInsertion{insertion: insertion, index: i})
	}
	for i := 1; i < len(insertions); i++ {
		key := insertions[i]
		j := i - 1
		for j >= 0 && (insertions[j].insertion.AfterLine < key.insertion.AfterLine || insertions[j].insertion.AfterLine == key.insertion.AfterLine && insertions[j].index < key.index) {
			insertions[j+1] = insertions[j]
			j--
		}
		insertions[j+1] = key
	}
	for _, item := range insertions {
		insertion := item.insertion
		if strings.TrimSpace(insertion.Markdown) == "" {
			continue
		}
		after := insertion.AfterLine
		if after < 0 {
			after = 0
		}
		if after > len(lines) {
			after = len(lines)
		}
		newLines := splitLines(strings.Trim(insertion.Markdown, "\n"))
		lines = append(lines[:after], append(newLines, lines[after:]...)...)
	}
	return strings.Join(lines, "\n")
}

func compactNote(path string, content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return "PATH: " + path + "\n" + content
	}
	return "PATH: " + path + "\n" + content[:maxChars] + "\n[truncated]"
}
