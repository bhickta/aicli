package training

import "strings"

func inspectNoteBoundaries(content string) (int, bool) {
	lines := strings.Split(content, "\n")
	var count int
	var open bool
	var bodyLines int
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "BEGIN_NOTE "):
			if open || strings.TrimSpace(strings.TrimPrefix(line, "BEGIN_NOTE ")) == "" {
				return count, true
			}
			open = true
			bodyLines = 0
		case line == "END_NOTE":
			if !open || bodyLines == 0 {
				return count, true
			}
			open = false
			count++
		case open:
			if line != "" {
				bodyLines++
			}
		case line != "":
			return count, true
		}
	}
	return count, open || count == 0
}

func hasDuplicateLeadingFrontmatter(content string) bool {
	blocks := strings.Split(content, "BEGIN_NOTE ")
	for _, block := range blocks[1:] {
		bodyStart := strings.IndexByte(block, '\n')
		if bodyStart < 0 {
			continue
		}
		body := strings.TrimLeft(block[bodyStart+1:], "\n")
		lines := strings.Split(body, "\n")
		if len(lines) < 4 || strings.TrimSpace(lines[0]) != "---" {
			continue
		}
		for i := 1; i < len(lines)-1; i++ {
			if strings.TrimSpace(lines[i]) != "---" {
				continue
			}
			if strings.TrimSpace(lines[i+1]) == "---" {
				return true
			}
			break
		}
	}
	return false
}

func hasStatusOrJSONOutput(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	upper := strings.ToUpper(trimmed)
	return strings.HasPrefix(trimmed, "{") ||
		strings.HasPrefix(trimmed, "[") ||
		strings.HasPrefix(upper, "PENDING") ||
		strings.HasPrefix(upper, "FAILED") ||
		strings.HasPrefix(upper, "ERROR")
}
