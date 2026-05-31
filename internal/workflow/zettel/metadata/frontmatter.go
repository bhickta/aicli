package metadata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

const frontmatterDelimiter = "---"

func applyMetadata(content string, item generatedMetadata, overwrite bool) (string, bool, bool) {
	frontmatter, body, hasFrontmatter := splitFrontmatter(content)
	if hasCompleteMetadata(frontmatter) && !overwrite {
		return content, false, true
	}

	frontmatter = renderMetadataFrontmatter(frontmatter, item)
	if !hasFrontmatter {
		body = strings.TrimLeft(content, "\n")
	}
	after := frontmatterDelimiter + "\n" + frontmatter + frontmatterDelimiter + "\n" + body
	after = notetext.EnsureTrailingNewline(after)
	return after, after != content, false
}

func splitFrontmatter(content string) (string, string, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, frontmatterDelimiter+"\n") {
		return "", content, false
	}
	rest := content[len(frontmatterDelimiter+"\n"):]
	end := strings.Index(rest, "\n"+frontmatterDelimiter)
	if end < 0 {
		return "", content, false
	}
	frontmatter := rest[:end]
	body := rest[end+len("\n"+frontmatterDelimiter):]
	body = strings.TrimPrefix(body, "\n")
	return notetext.EnsureTrailingNewline(frontmatter), body, true
}

func hasCompleteMetadata(frontmatter string) bool {
	title := false
	summary := false
	questions := 0
	inQuestions := false
	for _, line := range strings.Split(strings.ReplaceAll(frontmatter, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if !isIndented {
			inQuestions = false
		}
		switch {
		case strings.HasPrefix(trimmed, "title:") && strings.TrimSpace(strings.TrimPrefix(trimmed, "title:")) != "":
			title = true
		case strings.HasPrefix(trimmed, "summary_keywords:") && strings.TrimSpace(strings.TrimPrefix(trimmed, "summary_keywords:")) != "":
			summary = true
		case strings.HasPrefix(trimmed, "recall_questions:"):
			inQuestions = true
		case inQuestions && strings.HasPrefix(trimmed, "- ") && strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")) != "":
			questions++
		}
	}
	return title && summary && questions >= 3
}

func renderMetadataFrontmatter(frontmatter string, item generatedMetadata) string {
	lines := removeMetadataFields(strings.Split(strings.TrimRight(frontmatter, "\n"), "\n"))
	lines = trimTrailingBlankLines(lines)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		"title: "+yamlQuote(item.Title),
		"summary_keywords: "+yamlQuote(item.SummaryKeywords),
		"recall_questions:",
	)
	for _, question := range item.RecallQuestions {
		lines = append(lines, "  - "+yamlQuote(question))
	}
	return strings.Join(lines, "\n") + "\n"
}

func removeMetadataFields(lines []string) []string {
	out := make([]string, 0, len(lines))
	skipBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if skipBlock {
			if trimmed == "" || isIndented {
				continue
			}
			skipBlock = false
		}
		if isMetadataScalar(trimmed) {
			continue
		}
		if strings.HasPrefix(trimmed, "recall_questions:") {
			skipBlock = true
			continue
		}
		out = append(out, line)
	}
	return out
}

func isMetadataScalar(trimmed string) bool {
	return strings.HasPrefix(trimmed, "title:") ||
		strings.HasPrefix(trimmed, "summary_keywords:")
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func yamlQuote(value string) string {
	data, err := json.Marshal(strings.TrimSpace(value))
	if err != nil {
		return fmt.Sprintf("%q", strings.TrimSpace(value))
	}
	return string(data)
}
