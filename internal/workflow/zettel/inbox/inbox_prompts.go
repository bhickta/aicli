package inbox

import (
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

type inboxDestinationDecision struct {
	Claims       []InboxClaim                 `json:"claims,omitempty"`
	Destinations []inboxDestinationAssignment `json:"destinations"`
	Pending      []InboxClaimLedger           `json:"pending"`
}

type inboxDestinationAssignment struct {
	Path       string   `json:"path"`
	ClaimIDs   []string `json:"claim_ids,omitempty"`
	FinalNote  string   `json:"final_note,omitempty"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason,omitempty"`
}

type inboxDestinationJudgement struct {
	Targets       []string
	PendingReason string
}

type inboxValidationResult struct {
	OK     bool
	Pass   bool
	Reason string
}

func inboxJudgeMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) []provider.Message {
	user := strings.Join([]string{
		"SOURCE PATH:",
		sourcePath,
		"",
		"SOURCE NOTE:",
		sourceContent,
		"",
		"CANDIDATE DESTINATION NOTES:",
		formatInboxCandidateNotes(candidates, options),
	}, "\n")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You select the safest destination notes for a no-loss UPSC zettelkasten inbox merge.",
			"Return only TARGET lines or one PENDING line. Do not return JSON or commentary.",
			"Use this exact format for selected destinations:",
			"TARGET <candidate path>",
			"If no destination is safe, return: PENDING: <short reason>.",
			"Use as few targets as possible; one target is preferred.",
			"Use multiple targets only when the source note clearly contains separate atomic concepts.",
			"Every TARGET path must be one of the provided candidate paths.",
			"Prefer a candidate whose folder, subject, and concept match the source.",
			"Select the adopted new-note candidate when existing candidates are too narrow, wrong subject, or would make a note non-atomic.",
			"Do not put an overview, syllabus, strategy, or roadmap note into a narrow subtopic note.",
			"Do not select a target just because one word overlaps; preserve conceptual fit.",
			"",
			"STYLE CONTEXT:",
			shorthandPrompt,
		}, "\n")},
		{Role: "user", Content: user},
	}
}

func inboxMergeMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) []provider.Message {
	user := strings.Join([]string{
		"SOURCE PATH:",
		sourcePath,
		"",
		"SOURCE NOTE:",
		sourceContent,
		"",
		"APPROVED DESTINATION NOTES:",
		formatInboxCandidateNotes(candidates, options),
	}, "\n")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You perform a complete no-loss UPSC zettelkasten inbox merge.",
			"Return final destination notes only. Do not return JSON. Do not return line numbers. Do not return patch actions.",
			"Use this exact plain-text envelope for each destination you want rewritten:",
			"BEGIN_NOTE <candidate path>",
			"<complete final markdown for that note>",
			"END_NOTE",
			"If no destination is safe, return: PENDING: <short reason>.",
			"Each BEGIN_NOTE path must be one of the approved destination paths.",
			"Inside each note block, write the whole final note, not only the new lines.",
			"Keep existing YAML frontmatter exactly. For a new note, include YAML frontmatter with Status: Read.",
			"Use plain readable labels: `- **Concept Label**: fact`. Never use `::`, snake_case, underscore-separated headings, or vague abbreviations.",
			"Do not create generic labels such as `**Label**`, `**Topic**`, `**Heading**`, `**Title**`, or `**Note**`; write the real concept label.",
			"Preserve every existing destination fact unless it is an exact duplicate of the source merge.",
			"Preserve every useful source fact, number, qualifier, example, definition, and relationship.",
			"Preserve quoted phrases exactly, including every word inside quotation marks.",
			"Preserve contextual numbers exactly: years, percentages, rupee amounts, star ratings, marks, question references like Q1/Q2, hours, and stated counts.",
			"Markdown list numbering such as `1.` or `2.` is formatting, not a fact, unless the number is part of the concept label.",
			"Merge conceptually related source facts into the most appropriate final note.",
			"Do not merge a broad syllabus, roadmap, strategy, or overview note into a narrow destination about one subtopic.",
			"Use a candidate only when the final note remains atomic and coherent after the merge.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not add external knowledge.",
			"",
			"EXTREME SHORTHAND STYLE TEMPLATE:",
			shorthandPrompt,
		}, "\n")},
		{Role: "user", Content: string(user)},
	}
}

func inboxValidationMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, decision inboxDestinationDecision) []provider.Message {
	var finalNotes []string
	for _, destination := range decision.Destinations {
		finalNotes = append(finalNotes, strings.Join([]string{
			"BEGIN_NOTE " + destination.Path,
			destination.FinalNote,
			"END_NOTE",
		}, "\n"))
	}
	user := strings.Join([]string{
		"SOURCE PATH:",
		sourcePath,
		"",
		"SOURCE NOTE:",
		sourceContent,
		"",
		"APPROVED DESTINATION NOTES BEFORE MERGE:",
		formatInboxCandidateNotes(candidates, Options{MaxMergeInputChars: 60000}),
		"",
		"PROPOSED FINAL NOTES:",
		strings.Join(finalNotes, "\n\n"),
	}, "\n")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You are a strict UPSC zettelkasten merge validator.",
			"Return exactly PASS or FAIL: <short reason>. Do not return JSON, bullets, or commentary.",
			"PASS only if every useful source fact, number, qualifier, example, definition, and relationship is represented in the proposed final notes.",
			"PASS only if every existing destination fact is preserved unless it is an exact duplicate.",
			"FAIL if a quoted phrase from the source is paraphrased or removed.",
			"FAIL if contextual numbers are lost: years, percentages, rupee amounts, star ratings, marks, question references, hours, or stated counts.",
			"Ignore markdown list numbering such as `1.` or `2.` when it is only formatting.",
			"FAIL if the merge puts source facts into a wrong subject, wrong paper, wrong folder, or non-atomic destination.",
			"FAIL if an example or fact is duplicated across multiple final notes.",
			"FAIL if the final notes use cryptic shorthand such as `::`, `Sust Dev`, `Init`, `dims`, or snake_case.",
			"FAIL if the final notes use generic labels such as `**Label**`, `**Topic**`, `**Heading**`, `**Title**`, or `**Note**`.",
			"FAIL if the final notes add unsupported external knowledge.",
		}, "\n")},
		{Role: "user", Content: user},
	}
}

func formatInboxCandidateNotes(candidates []scoredCandidate, options Options) string {
	var candidateText []string
	charLimit := inboxCandidateCharLimit(options, len(candidates))
	for i, candidate := range candidates {
		excerpt, _ := notetext.NumberedExcerpt(candidate.Path, candidate.Content, charLimit)
		if strings.TrimSpace(candidate.Content) == "" {
			excerpt = "PATH: " + candidate.Path + "\n[new empty destination allowed]"
		}
		candidateText = append(candidateText, strings.Join([]string{
			fmt.Sprintf("CANDIDATE %d", i+1),
			"PATH: " + candidate.Path,
			fmt.Sprintf("SIMILARITY: %.4f", candidate.Similarity),
			"CURRENT NOTE:",
			excerpt,
		}, "\n"))
	}
	return strings.Join(candidateText, "\n\n")
}

func inboxCandidateCharLimit(options Options, count int) int {
	const minCandidateExcerptChars = 2500

	if count < 1 {
		count = 1
	}
	perCandidate := options.MaxMergeInputChars / count
	if perCandidate < minCandidateExcerptChars {
		return minCandidateExcerptChars
	}
	return perCandidate
}
