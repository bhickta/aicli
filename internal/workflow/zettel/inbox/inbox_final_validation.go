package inbox

import (
	"fmt"
	"strings"
)

func finalNotesValidation(sourceContent string, finalNotes map[string]string) MergeJudge {
	combined := strings.Builder{}
	for _, content := range finalNotes {
		combined.WriteString("\n")
		combined.WriteString(content)
	}
	finalTokenSet := topicTokenSet(combined.String())
	var missing []string
	for _, line := range sourceFactLines(sourceContent) {
		tokens := meaningfulClaimTokens(line)
		if len(tokens) == 0 {
			continue
		}
		matched := 0
		for _, token := range tokens {
			if finalTokenSet[token] {
				matched++
			}
		}
		if matched*10 < len(tokens)*7 {
			missing = append(missing, strings.TrimSpace(line))
		}
	}
	if len(missing) > 0 {
		return MergeJudge{
			Verdict:      "fail",
			Score:        0,
			MissingFacts: missing,
			Notes:        fmt.Sprintf("%d source line(s) were not represented in final note text", len(missing)),
		}
	}
	return MergeJudge{
		Verdict: "pass",
		Score:   1,
		Notes:   "Final note text mechanically covers source note facts.",
	}
}

func sourceFactLines(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || strings.HasPrefix(strings.ToLower(trimmed), "status:") {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
