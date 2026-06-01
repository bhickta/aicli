package training

import (
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

func inspectExample(example chatExample) exampleQuality {
	assistant := assistantContent(example)
	user := userContent(example)
	noteCount, badNoteBoundaries := inspectNoteBoundaries(assistant)
	quality := exampleQuality{
		systemHash:              hashText(systemContent(example)),
		hasSemanticCandidates:   strings.Contains(user, "SEMANTIC DESTINATION CANDIDATES:"),
		hasCodeFence:            strings.Contains(assistant, "```"),
		hasDuplicateFrontmatter: hasDuplicateLeadingFrontmatter(assistant),
		hasBadNoteBoundaries:    badNoteBoundaries,
		hasStatusOrJSONOutput:   hasStatusOrJSONOutput(assistant),
		finalNoteCount:          noteCount,
		userChars:               len(user),
		assistantChars:          len(assistant),
	}
	return quality
}

func strictSkipReason(quality exampleQuality, primarySystemHash string) string {
	switch {
	case quality.systemHash != primarySystemHash:
		return "non-primary-system-prompt"
	case !quality.hasSemanticCandidates:
		return "missing-semantic-candidates"
	case quality.hasCodeFence:
		return "assistant-code-fence"
	case quality.hasDuplicateFrontmatter:
		return "duplicate-frontmatter"
	case quality.hasBadNoteBoundaries:
		return "bad-note-boundaries"
	case quality.hasStatusOrJSONOutput:
		return "assistant-status-or-json"
	default:
		return ""
	}
}

func buildQualityReport(records []exportRecord) model.TrainingQualityReport {
	report := model.TrainingQualityReport{}
	systemHashes := map[string]int{}
	var userChars int
	var assistantChars int
	for i, record := range records {
		quality := record.quality
		systemHashes[quality.systemHash]++
		updateQualityFlags(&report, quality)
		report.TotalFinalNotes += quality.finalNoteCount
		if quality.finalNoteCount > report.MaxFinalNotesPerExample {
			report.MaxFinalNotesPerExample = quality.finalNoteCount
		}
		userChars += quality.userChars
		assistantChars += quality.assistantChars
		updateQualityExtremes(&report, quality, i == 0)
	}
	report.SystemPromptVariants = len(systemHashes)
	report.PrimarySystemPromptCount = mostCommonSystemPromptCount(systemHashes)
	if len(records) > 0 {
		report.AverageUserChars = userChars / len(records)
		report.AverageAssistantChars = assistantChars / len(records)
	}
	return report
}

func updateQualityFlags(report *model.TrainingQualityReport, quality exampleQuality) {
	if quality.hasSemanticCandidates {
		report.ExamplesWithSemanticCandidates++
	} else {
		report.ExamplesWithoutSemanticCandidates++
	}
	if quality.hasCodeFence {
		report.ExamplesWithCodeFences++
	}
	if quality.hasDuplicateFrontmatter {
		report.ExamplesWithDuplicateFrontmatter++
	}
	if quality.hasBadNoteBoundaries {
		report.ExamplesWithBadNoteBoundaries++
	}
	if quality.hasStatusOrJSONOutput {
		report.ExamplesWithStatusOrJSONOutput++
	}
	if quality.assistantChars < 500 {
		report.ShortAssistantCount++
	}
	if quality.assistantChars > 20000 {
		report.LongAssistantCount++
	}
}

func updateQualityExtremes(report *model.TrainingQualityReport, quality exampleQuality, first bool) {
	if first || quality.userChars < report.MinUserChars {
		report.MinUserChars = quality.userChars
	}
	if quality.userChars > report.MaxUserChars {
		report.MaxUserChars = quality.userChars
	}
	if first || quality.assistantChars < report.MinAssistantChars {
		report.MinAssistantChars = quality.assistantChars
	}
	if quality.assistantChars > report.MaxAssistantChars {
		report.MaxAssistantChars = quality.assistantChars
	}
}
