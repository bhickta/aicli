package notetext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func SplitLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

func EnsureTrailingNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func HashText(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func NumberedNote(path string, content string) string {
	lines := SplitLines(content)
	out := make([]string, 0, len(lines)+2)
	out = append(out, "PATH: "+path, "LINES:")
	for i, line := range lines {
		out = append(out, fmt.Sprintf("%4d | %s", i+1, line))
	}
	return strings.Join(out, "\n")
}

func NumberedExcerpt(path string, content string, maxChars int) (string, int) {
	numbered := NumberedNote(path, content)
	lines := strings.Split(numbered, "\n")
	if len(numbered) <= maxChars {
		return numbered, len(SplitLines(content))
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

func CompactNote(path string, content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return "PATH: " + path + "\n" + content
	}
	return "PATH: " + path + "\n" + content[:maxChars] + "\n[truncated]"
}
