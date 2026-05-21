package notetext

import "strings"

func SimpleMarkdownDiff(before string, after string) string {
	if before == after {
		return ""
	}
	beforeLines := SplitLines(before)
	afterLines := SplitLines(after)
	var out []string
	out = append(out, "--- before", "+++ after")
	maxLen := len(beforeLines)
	if len(afterLines) > maxLen {
		maxLen = len(afterLines)
	}
	for i := 0; i < maxLen; i++ {
		var beforeLine string
		var afterLine string
		if i < len(beforeLines) {
			beforeLine = beforeLines[i]
		}
		if i < len(afterLines) {
			afterLine = afterLines[i]
		}
		switch {
		case i >= len(beforeLines):
			out = append(out, "+"+afterLine)
		case i >= len(afterLines):
			out = append(out, "-"+beforeLine)
		case beforeLine == afterLine:
			out = append(out, " "+beforeLine)
		default:
			out = append(out, "-"+beforeLine, "+"+afterLine)
		}
	}
	return strings.Join(out, "\n")
}
