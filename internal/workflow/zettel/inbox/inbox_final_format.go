package inbox

import (
	"regexp"
	"strings"
)

var (
	doubleColonLabelPattern = regexp.MustCompile(`\*\*([^*\n]+)\*\*::`)
	snakeTokenPattern       = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*(?:_[A-Za-z0-9]+)+\b`)
)

func formatFinalNoteMarkdown(before string, markdown string) string {
	markdown = strings.Trim(markdown, "\n")
	markdown = normalizeFinalNoteStyle(markdown)
	if existing := frontmatterBlock(before); existing != "" {
		return strings.TrimRight(existing, "\n") + "\n" + stripFrontmatter(markdown)
	}
	if hasFrontmatter(markdown) {
		return markdown
	}
	frontmatter := "---\nStatus: Read\n---\n"
	return strings.TrimRight(frontmatter, "\n") + "\n" + strings.TrimLeft(markdown, "\n")
}

func normalizeFinalNoteStyle(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = doubleColonLabelPattern.ReplaceAllString(markdown, "**$1**:")
	return snakeTokenPattern.ReplaceAllStringFunc(markdown, func(value string) string {
		return strings.ReplaceAll(value, "_", " ")
	})
}

func hasFrontmatter(markdown string) bool {
	return strings.HasPrefix(strings.TrimLeft(markdown, "\n"), "---\n")
}

func frontmatterBlock(markdown string) string {
	markdown = strings.ReplaceAll(strings.TrimLeft(markdown, "\n"), "\r\n", "\n")
	if !strings.HasPrefix(markdown, "---\n") {
		return ""
	}
	rest := markdown[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	return "---\n" + rest[:end] + "\n---\n"
}

func stripFrontmatter(markdown string) string {
	markdown = strings.ReplaceAll(strings.TrimLeft(markdown, "\n"), "\r\n", "\n")
	if !strings.HasPrefix(markdown, "---\n") {
		return strings.TrimLeft(markdown, "\n")
	}
	rest := markdown[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return strings.TrimLeft(markdown, "\n")
	}
	return strings.TrimLeft(rest[end+len("\n---"):], "\n")
}
