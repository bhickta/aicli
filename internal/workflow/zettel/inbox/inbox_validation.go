package inbox

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	validationNumberPattern = regexp.MustCompile(`\b\d+(?:[.,]\d+)*(?:\.\d+)?(?:%|[a-zA-Z]+)?\b`)
	validationQuotePattern  = regexp.MustCompile(`["']([^"']{2,120})["']`)
)

func mechanicalInboxValidation(applied bool) MergeJudge {
	if !applied {
		return MergeJudge{}
	}
	return MergeJudge{
		Verdict: "pass",
		Score:   1,
		Notes:   "Mechanical adoption: destination_after is the full source note copied into a new zettelkasten note.",
	}
}

func constrainFinalNoteRoutes(decision inboxDestinationDecision, sourceContent string) inboxDestinationDecision {
	if !decision.FinalNotes {
		return decision
	}
	out := decision
	out.Destinations = nil
	var rejected []InboxClaimLedger
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		if finalNoteDestinationFitsSource(path, sourceContent) {
			out.Destinations = append(out.Destinations, destination)
			continue
		}
		reason := fmt.Sprintf("destination topic is too narrow or mismatched for source note: %s", path)
		for _, claimID := range destinationClaimIDs(destination) {
			rejected = append(rejected, InboxClaimLedger{
				ClaimID:         claimID,
				Status:          claimStatusPending,
				DestinationPath: path,
				Reason:          reason,
			})
		}
	}
	if len(out.Destinations) == 0 {
		out.Pending = append(out.Pending, rejected...)
	}
	return out
}

func finalNoteDestinationFitsSource(path string, sourceContent string) bool {
	destinationTokens := strongDestinationTokens(path)
	if len(destinationTokens) == 0 {
		return true
	}
	sourceTokens := normalizedTopicTokens(sourceContent)
	for _, destinationToken := range destinationTokens {
		for _, sourceToken := range sourceTokens {
			if relatedTopicTokens(destinationToken, sourceToken) {
				return true
			}
		}
	}
	return false
}

func finalNoteInboxValidation(sourceContent string, applied inboxAppliedDecision) MergeJudge {
	finalContent := joinedDestinationContent(applied.destinationAfter)
	if strings.TrimSpace(finalContent) == "" {
		return MergeJudge{
			Verdict:      "fail",
			Score:        0,
			MissingFacts: []string{"no final destination note content was produced"},
			Notes:        "final-note validation failed: no destination content",
		}
	}
	missing := missingSourceFacts(sourceContent, finalContent)
	unsupported := unsupportedFinalNoteAdditions(sourceContent, applied.destinationBefore, applied.destinationAfter)
	if len(missing) > 0 || len(unsupported) > 0 {
		return MergeJudge{
			Verdict:              "fail",
			Score:                validationScore(len(sourceFactUnits(sourceContent)), len(missing)),
			MissingFacts:         missing,
			UnsupportedAdditions: unsupported,
			Notes:                validationFailureNotes(missing, unsupported),
		}
	}
	return MergeJudge{
		Verdict: "pass",
		Score:   1,
		Notes:   "Final-note validation passed: source facts are represented and added lines are supported.",
	}
}

func joinedDestinationContent(destinationAfter map[string]string) string {
	parts := make([]string, 0, len(destinationAfter))
	for _, content := range destinationAfter {
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n")
}

func missingSourceFacts(sourceContent string, finalContent string) []string {
	var missing []string
	for _, fact := range sourceFactUnits(sourceContent) {
		if !factSupportedByText(fact, finalContent) {
			missing = append(missing, fact)
		}
	}
	return missing
}

func unsupportedFinalNoteAdditions(sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string) []string {
	var unsupported []string
	for path, after := range destinationAfter {
		before := destinationBefore[path]
		for _, line := range addedSubstantiveLines(before, after) {
			if factSupportedByText(line, sourceContent) || factSupportedByText(line, before) {
				continue
			}
			unsupported = append(unsupported, fmt.Sprintf("%s: %s", path, line))
		}
	}
	return unsupported
}

func addedSubstantiveLines(before string, after string) []string {
	beforeCounts := map[string]int{}
	for _, line := range strings.Split(stripFrontmatter(before), "\n") {
		cleaned := cleanFactLine(line)
		if !substantiveFact(cleaned) {
			continue
		}
		beforeCounts[normalizeFactText(cleaned)]++
	}
	var added []string
	for _, line := range strings.Split(stripFrontmatter(after), "\n") {
		cleaned := cleanFactLine(line)
		if !substantiveFact(cleaned) {
			continue
		}
		key := normalizeFactText(cleaned)
		if beforeCounts[key] > 0 {
			beforeCounts[key]--
			continue
		}
		added = append(added, cleaned)
	}
	return added
}

func sourceFactUnits(content string) []string {
	var facts []string
	for _, line := range strings.Split(stripFrontmatter(content), "\n") {
		cleaned := cleanFactLine(line)
		if substantiveFact(cleaned) {
			facts = append(facts, cleaned)
		}
	}
	return facts
}

func cleanFactLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	for {
		next := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"), "+"))
		if next == line {
			break
		}
		line = next
	}
	line = strings.TrimPrefix(line, ">")
	return strings.TrimSpace(line)
}

func substantiveFact(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 8 {
		return false
	}
	if line == "---" || strings.HasPrefix(line, "```") {
		return false
	}
	if strings.EqualFold(line, "Status: Read") {
		return false
	}
	return true
}

func factSupportedByText(fact string, text string) bool {
	fact = cleanFactLine(fact)
	if !substantiveFact(fact) {
		return true
	}
	if missingRequiredArtifacts(fact, text) {
		return false
	}
	normalizedFact := normalizeFactText(fact)
	normalizedText := normalizeFactText(text)
	if len(normalizedFact) >= 12 && strings.Contains(normalizedText, normalizedFact) {
		return true
	}
	tokens := meaningfulClaimTokens(fact)
	if len(tokens) == 0 {
		return true
	}
	textTokens := topicTokenSet(text)
	matched := 0
	for _, token := range tokens {
		if textTokens[token] {
			matched++
		}
	}
	if len(tokens) <= 4 {
		return matched == len(tokens)
	}
	return matched*10 >= len(tokens)*8
}

func missingRequiredArtifacts(fact string, text string) bool {
	textLower := strings.ToLower(text)
	for _, number := range validationNumberPattern.FindAllString(fact, -1) {
		if !strings.Contains(textLower, strings.ToLower(number)) {
			return true
		}
	}
	for _, match := range validationQuotePattern.FindAllStringSubmatch(fact, -1) {
		phrase := strings.TrimSpace(match[1])
		if phrase != "" && !strings.Contains(textLower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func normalizeFactText(value string) string {
	tokens := normalizedTopicTokens(value)
	return strings.Join(tokens, " ")
}

func validationScore(totalFacts int, missingFacts int) float64 {
	if totalFacts <= 0 {
		return 0
	}
	score := float64(totalFacts-missingFacts) / float64(totalFacts)
	if score < 0 {
		return 0
	}
	return score
}

func validationFailureNotes(missing []string, unsupported []string) string {
	switch {
	case len(missing) > 0 && len(unsupported) > 0:
		return fmt.Sprintf("final-note validation failed: %d source facts missing and %d unsupported additions", len(missing), len(unsupported))
	case len(missing) > 0:
		return fmt.Sprintf("final-note validation failed: %d source facts missing", len(missing))
	default:
		return fmt.Sprintf("final-note validation failed: %d unsupported additions", len(unsupported))
	}
}
